package core

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	providerapi "github.com/jedarden/tunnel/internal/providers"
)

// failover_statemachine_test.go pins failover state-machine transitions that
// the determinism suite in failover_behavior_test.go leaves uncovered:
// rotation inside one priority tier, auto-recovery disabled, promotion of a
// recovered provider decided from recorded health without a fresh reachability
// probe, tunnel reachability gating the health counters, failover deferred
// while no backup is healthy, latency gating, and bootstrap selection. Every
// test is hermetic: outcomes are driven by stubbed provider health and seeded
// metrics, with no transport probe and no live monitor loop.

// probeCountingProvider is a healthStubProvider that also records how many
// times the manager consulted HealthCheck, so a test can tell check-time
// probing apart from decisions made purely from recorded health state.
type probeCountingProvider struct {
	healthStubProvider
	probes atomic.Int64
}

func (p *probeCountingProvider) HealthCheck() (*providerapi.HealthStatus, error) {
	p.probes.Add(1)
	return &providerapi.HealthStatus{Healthy: p.healthy}, nil
}

// TestSamePriorityTierRotatesWithoutRecoveredPeerReclaim pins same-priority
// rotation: when the primary fails, promotion moves to the next deterministic
// peer in the same tier; a recovered peer must NOT reclaim at equal priority
// (auto-recovery promotes strictly higher priority only, so the tier must not
// flap between equals); and a later failure rotates back within the tier.
func TestSamePriorityTierRotatesWithoutRecoveredPeerReclaim(t *testing.T) {
	cfg := behaviorTestConfig()
	tierStubs := map[string]*healthStubProvider{
		"tier-a-svc": {name: "tier-a-svc", healthy: true},
		"tier-b-svc": {name: "tier-b-svc", healthy: true},
		"tier-c-svc": {name: "tier-c-svc", healthy: true},
	}
	providers := make(map[string]ProviderProvider, len(tierStubs))
	for method, stub := range tierStubs {
		providers[method] = stub
	}
	fm := NewFailoverManager(cfg, NewEventPublisher(64), nil, providers, nil)

	tierA := newBehaviorConn("tier-a", "tier-a-svc", 1)
	tierB := newBehaviorConn("tier-b", "tier-b-svc", 1)
	tierC := newBehaviorConn("tier-c", "tier-c-svc", 1)
	registerBehaviorConns(t, fm, cfg, tierA, tierB, tierC)
	if err := fm.SetPrimary(tierA.ID); err != nil {
		t.Fatalf("SetPrimary(tier-a) failed: %v", err)
	}

	// tier-a fails through the threshold: the tier rotates to the next
	// deterministic peer (lowest ID among healthy equals), never outside the tie.
	tierStubs["tier-a-svc"].healthy = false
	for i := 1; i <= cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
		if i < cfg.FailureThreshold && fm.GetPrimary() != tierA.ID {
			t.Fatalf("rotation left tier-a after %d of %d failed checks, primary=%q",
				i, cfg.FailureThreshold, fm.GetPrimary())
		}
	}
	if fm.GetPrimary() != tierB.ID {
		t.Fatalf("same-priority rotation landed on %q, want tier-b (lowest healthy ID in the tier)", fm.GetPrimary())
	}
	if tierA.IsPrimaryConnection() || !tierB.IsPrimaryConnection() {
		t.Fatalf("primary flags after rotation: tier-a=%v tier-b=%v, want tier-a=false tier-b=true",
			tierA.IsPrimaryConnection(), tierB.IsPrimaryConnection())
	}

	// tier-a recovers to full health: an equal-priority peer must not reclaim
	// the primary role, or the tier would flap whenever a member hiccups.
	tierStubs["tier-a-svc"].healthy = true
	for i := 0; i < cfg.RecoveryThreshold; i++ {
		fm.performHealthChecks()
	}
	if snap := snapshotHealth(t, fm, tierA.ID); !snap.healthy {
		t.Fatalf("precondition: tier-a did not recover to healthy (snapshot %+v)", snap)
	}
	if fm.GetPrimary() != tierB.ID {
		t.Fatalf("recovered same-priority peer reclaimed the primary (primary=%q); equal priority must not steal back", fm.GetPrimary())
	}

	// tier-b fails in turn: the rotation lands back on the recovered tier-a —
	// the lowest healthy ID ahead of tier-c — completing the tier rotation.
	tierStubs["tier-b-svc"].healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
	}
	if fm.GetPrimary() != tierA.ID {
		t.Fatalf("second rotation landed on %q, want tier-a (rotation back within the tier)", fm.GetPrimary())
	}
	if !tierA.IsPrimaryConnection() || tierB.IsPrimaryConnection() {
		t.Fatalf("primary flags after second rotation: tier-a=%v tier-b=%v, want tier-a=true tier-b=false",
			tierA.IsPrimaryConnection(), tierB.IsPrimaryConnection())
	}
}

// TestAutoRecoverDisabledSuppressesRecoveryPromotion pins the AutoRecover=off
// branch: a recovered higher-priority provider must NOT be promoted while the
// current primary stays healthy, and no promotion attempt may be made at all;
// recovery to the higher priority still happens through the failover path
// once the current primary itself fails.
func TestAutoRecoverDisabledSuppressesRecoveryPromotion(t *testing.T) {
	cfg := behaviorTestConfig()
	cfg.AutoRecover = false
	primaryStub := &healthStubProvider{name: "primary-svc", healthy: true}
	backupStub := &healthStubProvider{name: "backup-svc", healthy: true}
	fm := NewFailoverManager(cfg, NewEventPublisher(64), nil, map[string]ProviderProvider{
		primaryStub.Name(): primaryStub,
		backupStub.Name():  backupStub,
	}, nil)

	primary := newBehaviorConn("primary", primaryStub.Name(), 0)
	backup := newBehaviorConn("backup", backupStub.Name(), 1)
	registerBehaviorConns(t, fm, cfg, primary, backup)
	if err := fm.SetPrimary(primary.ID); err != nil {
		t.Fatalf("SetPrimary(primary) failed: %v", err)
	}

	subscriber := fm.eventPublisher.Subscribe("all", func(*ConnectionEvent) bool { return true })

	// Failover itself must keep working with auto-recovery disabled.
	primaryStub.healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
	}
	if fm.GetPrimary() != backup.ID {
		t.Fatalf("primary = %q with AutoRecover disabled, want failover to %q", fm.GetPrimary(), backup.ID)
	}
	for _, want := range []EventType{EventError, EventFailover} {
		if event := waitForEvent(t, subscriber.Channel); event.Type != want {
			t.Fatalf("event = %v, want %v", event.Type, want)
		}
	}

	// The higher-priority primary recovers: no promotion while the current
	// primary is healthy, and no primary-change event of any kind.
	primaryStub.healthy = true
	for i := 0; i < cfg.RecoveryThreshold+1; i++ {
		fm.performHealthChecks()
	}
	if snap := snapshotHealth(t, fm, primary.ID); !snap.healthy {
		t.Fatalf("precondition: primary did not recover to healthy (snapshot %+v)", snap)
	}
	if fm.GetPrimary() != backup.ID {
		t.Fatalf("recovered provider was promoted with AutoRecover disabled: primary=%q, want %q held",
			fm.GetPrimary(), backup.ID)
	}
	assertNoEvent(t, subscriber.Channel)

	// When the current primary fails too, the failover path selects the best
	// candidate — now the recovered higher-priority connection — so control
	// still returns to it.
	backupStub.healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
	}
	if fm.GetPrimary() != primary.ID {
		t.Fatalf("after current primary failed, primary = %q, want recovery to higher-priority %q through the failover path",
			fm.GetPrimary(), primary.ID)
	}
}

// TestRecoveredProviderPromotionUsesRecordedHealthOnly pins that promotion of
// a recovered provider is decided from the threshold-gated health record
// alone: the promotion step must not consult a fresh reachability probe, so
// the decision works with no transport involved at decision time.
func TestRecoveredProviderPromotionUsesRecordedHealthOnly(t *testing.T) {
	cfg := behaviorTestConfig()
	currentStub := &probeCountingProvider{healthStubProvider: healthStubProvider{name: "current-svc", healthy: true}}
	goldStub := &probeCountingProvider{healthStubProvider: healthStubProvider{name: "gold-svc", healthy: true}}
	fm := NewFailoverManager(cfg, NewEventPublisher(16), nil, map[string]ProviderProvider{
		"current-svc": currentStub,
		"gold-svc":    goldStub,
	}, nil)

	current := newBehaviorConn("current", "current-svc", 5)
	gold := newBehaviorConn("gold", "gold-svc", 1)
	registerBehaviorConns(t, fm, cfg, current, gold)
	if err := fm.SetPrimary(current.ID); err != nil {
		t.Fatalf("SetPrimary(current) failed: %v", err)
	}

	if snap := snapshotHealth(t, fm, gold.ID); !snap.healthy {
		t.Fatalf("precondition: gold did not hold a healthy record (snapshot %+v)", snap)
	}

	// Promotion reads the recorded health state machine output only.
	before := goldStub.probes.Load()
	fm.checkForBetterPrimary(current.ID)
	if after := goldStub.probes.Load(); after != before {
		t.Fatalf("promotion consulted %d reachability probe(s); promotion must be decided from recorded health only",
			after-before)
	}
	if fm.GetPrimary() != gold.ID {
		t.Fatalf("recovered higher-priority provider was not promoted: primary=%q, want %q", fm.GetPrimary(), gold.ID)
	}
	if !gold.IsPrimaryConnection() || current.IsPrimaryConnection() {
		t.Fatalf("primary flags after promotion: gold=%v current=%v, want gold=true current=false",
			gold.IsPrimaryConnection(), current.IsPrimaryConnection())
	}
}

// TestDisconnectedTunnelFailsChecksDespiteHealthyProvider pins that tunnel
// reachability gates the health counters independently of provider-reported
// health: while the connection is not in StateConnected, every check counts
// as a failure even when the provider reports healthy, and recovery only
// accumulates once the tunnel state is connected again.
func TestDisconnectedTunnelFailsChecksDespiteHealthyProvider(t *testing.T) {
	cfg := behaviorTestConfig()
	stub := &healthStubProvider{name: "stub-svc", healthy: true}
	fm := NewFailoverManager(cfg, NewEventPublisher(16), nil, map[string]ProviderProvider{
		stub.Name(): stub,
	}, nil)
	conn := newBehaviorConn("conn-1", stub.Name(), 0)
	fm.RegisterConnection(conn)
	runSuccessChecks(fm, conn, cfg.RecoveryThreshold)

	// Tunnel down, provider happy: failures still accumulate.
	conn.SetState(StateDisconnected)
	for i := 1; i <= cfg.FailureThreshold; i++ {
		fm.checkConnection(conn)
		snap := snapshotHealth(t, fm, conn.ID)
		if i < cfg.FailureThreshold {
			if !snap.healthy || snap.failures != i || snap.successes != 0 {
				t.Fatalf("after %d unreachable checks: healthy=%v failures=%d successes=%d, want healthy with failures=%d successes=0",
					i, snap.healthy, snap.failures, snap.successes, i)
			}
		}
	}
	if snap := snapshotHealth(t, fm, conn.ID); snap.healthy || snap.failures != cfg.FailureThreshold {
		t.Fatalf("after %d unreachable checks: healthy=%v failures=%d, want unhealthy with failures=%d",
			cfg.FailureThreshold, snap.healthy, snap.failures, cfg.FailureThreshold)
	}

	// Any non-connected state keeps failing checks, reconnecting included.
	conn.SetState(StateReconnecting)
	fm.checkConnection(conn)
	if snap := snapshotHealth(t, fm, conn.ID); snap.healthy || snap.failures != cfg.FailureThreshold+1 {
		t.Fatalf("check while reconnecting: healthy=%v failures=%d, want unhealthy failures=%d",
			snap.healthy, snap.failures, cfg.FailureThreshold+1)
	}

	// Tunnel restored: successes accumulate and the recovery threshold gates
	// the healthy flag again.
	conn.SetState(StateConnected)
	for i := 1; i <= cfg.RecoveryThreshold; i++ {
		fm.checkConnection(conn)
		snap := snapshotHealth(t, fm, conn.ID)
		if i < cfg.RecoveryThreshold {
			if snap.healthy || snap.successes != i || snap.failures != 0 {
				t.Fatalf("after %d restored checks: healthy=%v successes=%d failures=%d, want unhealthy with successes=%d failures=0",
					i, snap.healthy, snap.successes, snap.failures, i)
			}
		}
	}
	if snap := snapshotHealth(t, fm, conn.ID); !snap.healthy || snap.successes != cfg.RecoveryThreshold {
		t.Fatalf("after %d restored checks: healthy=%v successes=%d, want healthy with successes=%d",
			cfg.RecoveryThreshold, snap.healthy, snap.successes, cfg.RecoveryThreshold)
	}
}

// TestNoHealthyBackupDefersFailoverUntilBackupRecovers pins the no-backup
// branch: with the only backup itself unhealthy, a failing primary keeps the
// primary role — nothing unhealthy is promoted and the pool is not left
// leaderless — and the no-backup error event is published. The failure is not
// forgotten: once the backup recovers, the still-failing primary fails over.
func TestNoHealthyBackupDefersFailoverUntilBackupRecovers(t *testing.T) {
	cfg := behaviorTestConfig()
	primaryStub := &healthStubProvider{name: "primary-svc", healthy: true}
	backupStub := &healthStubProvider{name: "backup-svc", healthy: true}
	fm := NewFailoverManager(cfg, NewEventPublisher(64), nil, map[string]ProviderProvider{
		primaryStub.Name(): primaryStub,
		backupStub.Name():  backupStub,
	}, nil)

	primary := newBehaviorConn("primary", primaryStub.Name(), 0)
	backup := newBehaviorConn("backup", backupStub.Name(), 1)
	registerBehaviorConns(t, fm, cfg, primary, backup)
	if err := fm.SetPrimary(primary.ID); err != nil {
		t.Fatalf("SetPrimary(primary) failed: %v", err)
	}

	// The only backup loses its healthy record through real checks.
	backupStub.healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.checkConnection(backup)
	}
	if snap := snapshotHealth(t, fm, backup.ID); snap.healthy {
		t.Fatal("precondition: backup still healthy after failure threshold")
	}

	subscriber := fm.eventPublisher.Subscribe("all", func(*ConnectionEvent) bool { return true })

	primaryStub.healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
	}
	if fm.GetPrimary() != primary.ID {
		t.Fatalf("primary = %q with no healthy backup available, want %q kept", fm.GetPrimary(), primary.ID)
	}
	if !primary.IsPrimaryConnection() || backup.IsPrimaryConnection() {
		t.Fatalf("primary flags without a healthy backup: primary=%v backup=%v, want primary=true backup=false",
			primary.IsPrimaryConnection(), backup.IsPrimaryConnection())
	}

	// Error events repeat on every at-threshold check for either connection,
	// so scan until the dedicated no-backup event arrives.
	var noBackupEvent *ConnectionEvent
	for {
		event := waitForEvent(t, subscriber.Channel)
		if event.Type == EventError && strings.Contains(event.Message, "no healthy backup available") {
			noBackupEvent = event
			break
		}
		if event.Type != EventError {
			t.Fatalf("unexpected event %v while waiting for the no-backup error", event.Type)
		}
	}
	if noBackupEvent.ConnID != primary.ID || noBackupEvent.Data != nil {
		t.Fatalf("no-backup event = conn %s data %#v, want conn %s with no data",
			noBackupEvent.ConnID, noBackupEvent.Data, primary.ID)
	}
	assertNoEvent(t, subscriber.Channel)

	// The deferred failover: the backup recovers while the primary keeps
	// failing, and the very next evaluation promotes it.
	backupStub.healthy = true
	runSuccessChecks(fm, backup, cfg.RecoveryThreshold)
	fm.performHealthChecks()
	if fm.GetPrimary() != backup.ID {
		t.Fatalf("deferred failover landed on %q, want recovered %q", fm.GetPrimary(), backup.ID)
	}
	if !backup.IsPrimaryConnection() || primary.IsPrimaryConnection() {
		t.Fatalf("primary flags after deferred failover: primary=%v backup=%v, want primary=false backup=true",
			primary.IsPrimaryConnection(), backup.IsPrimaryConnection())
	}
}

// TestLatencyThresholdGatesHealthAndRecovery pins the MaxLatency knob of the
// health state machine: a check with latency exactly at the configured limit
// still succeeds, latency above the limit counts as failure despite a healthy
// provider and a connected tunnel, and returning under the limit recovers
// through the success threshold.
func TestLatencyThresholdGatesHealthAndRecovery(t *testing.T) {
	cfg := behaviorTestConfig()
	cfg.MaxLatency = 100 * time.Millisecond
	stub := &healthStubProvider{name: "stub-svc", healthy: true}
	collector := NewMetricsCollector(nil, false)
	fm := NewFailoverManager(cfg, NewEventPublisher(16), collector, map[string]ProviderProvider{
		stub.Name(): stub,
	}, nil)

	conn := newBehaviorConn("conn-1", stub.Name(), 0)
	fm.RegisterConnection(conn)
	collector.RegisterConnection(conn)
	runSuccessChecks(fm, conn, cfg.RecoveryThreshold)

	// At-limit latency is acceptable.
	conn.Metrics.Update(0, 0, cfg.MaxLatency)
	fm.checkConnection(conn)
	if snap := snapshotHealth(t, fm, conn.ID); !snap.healthy || snap.failures != 0 {
		t.Fatalf("at-limit latency check: healthy=%v failures=%d, want healthy with failures=0",
			snap.healthy, snap.failures)
	}

	// Above-limit latency fails checks until the failure threshold trips.
	conn.Metrics.Update(0, 0, cfg.MaxLatency+50*time.Millisecond)
	for i := 1; i <= cfg.FailureThreshold; i++ {
		fm.checkConnection(conn)
		snap := snapshotHealth(t, fm, conn.ID)
		if i < cfg.FailureThreshold {
			if !snap.healthy || snap.failures != i || snap.successes != 0 {
				t.Fatalf("above-limit check %d: healthy=%v failures=%d successes=%d, want healthy with failures=%d",
					i, snap.healthy, snap.failures, snap.successes, i)
			}
		}
	}
	if snap := snapshotHealth(t, fm, conn.ID); snap.healthy || snap.failures != cfg.FailureThreshold {
		t.Fatalf("above-limit checks exhausted: healthy=%v failures=%d, want unhealthy with failures=%d",
			snap.healthy, snap.failures, cfg.FailureThreshold)
	}

	// Latency back under the limit recovers through the success threshold.
	conn.Metrics.Update(0, 0, cfg.MaxLatency/2)
	for i := 1; i <= cfg.RecoveryThreshold; i++ {
		fm.checkConnection(conn)
		snap := snapshotHealth(t, fm, conn.ID)
		if i < cfg.RecoveryThreshold {
			if snap.healthy || snap.successes != i || snap.failures != 0 {
				t.Fatalf("latency recovery check %d: healthy=%v successes=%d failures=%d, want unhealthy with successes=%d failures=0",
					i, snap.healthy, snap.successes, snap.failures, i)
			}
		}
	}
	if snap := snapshotHealth(t, fm, conn.ID); !snap.healthy || snap.successes != cfg.RecoveryThreshold {
		t.Fatalf("latency recovery: healthy=%v successes=%d, want healthy with successes=%d",
			snap.healthy, snap.successes, cfg.RecoveryThreshold)
	}
}

// TestBootstrapSelectionWithoutPrimaryPromotesBestCandidate pins the
// bootstrap path: a pool that never had a primary selects one on the first
// evaluation cycle, choosing the highest-priority healthy connection even
// when its connection ID sorts last.
func TestBootstrapSelectionWithoutPrimaryPromotesBestCandidate(t *testing.T) {
	cfg := behaviorTestConfig()
	stubs := map[string]*healthStubProvider{
		"bravo-svc": {name: "bravo-svc", healthy: true},
		"zulu-svc":  {name: "zulu-svc", healthy: true},
	}
	providers := make(map[string]ProviderProvider, len(stubs))
	for method, stub := range stubs {
		providers[method] = stub
	}
	fm := NewFailoverManager(cfg, NewEventPublisher(16), nil, providers, nil)

	// zulu carries the highest priority but the lexicographically last ID:
	// priority must decide the bootstrap, not ID order.
	zulu := newBehaviorConn("zulu", "zulu-svc", 1)
	bravo := newBehaviorConn("bravo", "bravo-svc", 2)
	registerBehaviorConns(t, fm, cfg, zulu, bravo)
	if fm.GetPrimary() != "" {
		t.Fatalf("precondition: pool bootstrapped a primary %q before any evaluation", fm.GetPrimary())
	}

	subscriber := fm.eventPublisher.Subscribe("all", func(*ConnectionEvent) bool { return true })

	fm.performHealthChecks()

	if fm.GetPrimary() != zulu.ID {
		t.Fatalf("bootstrap selected %q, want highest-priority %q", fm.GetPrimary(), zulu.ID)
	}
	if !zulu.IsPrimaryConnection() || bravo.IsPrimaryConnection() {
		t.Fatalf("primary flags after bootstrap: zulu=%v bravo=%v, want zulu=true bravo=false",
			zulu.IsPrimaryConnection(), bravo.IsPrimaryConnection())
	}
	event := waitForEvent(t, subscriber.Channel)
	if event.Type != EventPrimaryChange || event.ConnID != zulu.ID {
		t.Fatalf("bootstrap event = %v for %s, want EventPrimaryChange for %s",
			event.Type, event.ConnID, zulu.ID)
	}
	assertNoEvent(t, subscriber.Channel)
}
