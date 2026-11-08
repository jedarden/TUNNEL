package core

import (
	"testing"
	"time"

	providerapi "github.com/jedarden/tunnel/internal/providers"
)

// healthStubProvider is a deterministic provider whose health the test
// controls directly: every check outcome flows from the healthy flag through
// the provider's own HealthCheck, with no transport probe involved.
type healthStubProvider struct {
	name    string
	healthy bool
}

func (p *healthStubProvider) Name() string { return p.name }

func (p *healthStubProvider) IsHealthy(*Connection) bool { return p.healthy }

func (p *healthStubProvider) HealthCheck() (*providerapi.HealthStatus, error) {
	return &providerapi.HealthStatus{Healthy: p.healthy}, nil
}

// legacyHealthStubProvider predates the richer HealthCheck interface, so the
// failover manager must rely on IsHealthy alone for this provider.
type legacyHealthStubProvider struct {
	name    string
	healthy bool
}

func (p *legacyHealthStubProvider) Name() string { return p.name }

func (p *legacyHealthStubProvider) IsHealthy(*Connection) bool { return p.healthy }

// behaviorTestConfig returns failover thresholds used by the behavior tests.
// The monitor loop is never started: every check below is driven manually, so
// the outcomes are deterministic and independent of wall-clock scheduling.
func behaviorTestConfig() *FailoverConfig {
	config := DefaultFailoverConfig()
	config.HealthCheckInterval = time.Hour
	config.FailureThreshold = 3
	config.RecoveryThreshold = 2
	config.AutoRecover = true
	return config
}

func newBehaviorConn(id, method string, priority int) *Connection {
	conn := NewConnection(id, method, 0, "", 0)
	conn.SetState(StateConnected)
	conn.SetPriority(priority)
	return conn
}

func runSuccessChecks(fm *FailoverManager, conn *Connection, count int) {
	for i := 0; i < count; i++ {
		fm.checkConnection(conn)
	}
}

// healthSnapshot captures the health counters that drive failover decisions.
type healthSnapshot struct {
	healthy   bool
	failures  int
	successes int
}

func snapshotHealth(t *testing.T, fm *FailoverManager, connID string) healthSnapshot {
	t.Helper()
	status, err := fm.GetHealthStatus(connID)
	if err != nil {
		t.Fatalf("GetHealthStatus(%s) failed: %v", connID, err)
	}
	status.mu.RLock()
	defer status.mu.RUnlock()
	return healthSnapshot{
		healthy:   status.IsHealthy,
		failures:  status.ConsecutiveFailures,
		successes: status.ConsecutiveSuccesses,
	}
}

func waitForEvent(t *testing.T, ch <-chan *ConnectionEvent) *ConnectionEvent {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event")
		return nil
	}
}

func assertNoEvent(t *testing.T, ch <-chan *ConnectionEvent) {
	t.Helper()
	select {
	case event := <-ch:
		t.Fatalf("unexpected extra event %v for %s", event.Type, event.ConnID)
	default:
	}
}

func registerBehaviorConns(t *testing.T, fm *FailoverManager, cfg *FailoverConfig, conns ...*Connection) {
	t.Helper()
	for _, conn := range conns {
		fm.RegisterConnection(conn)
		runSuccessChecks(fm, conn, cfg.RecoveryThreshold)
	}
}

func TestFindBestBackupPrefersLowerPriorityNumber(t *testing.T) {
	cfg := behaviorTestConfig()
	providers := make(map[string]ProviderProvider)
	fm := NewFailoverManager(cfg, NewEventPublisher(16), nil, providers, nil)

	// IDs deliberately disagree with priority order: the priority number must
	// decide, not lexicographic ID order or map iteration order.
	zeta := newBehaviorConn("zeta", "zeta-svc", 1) // highest preference
	alpha := newBehaviorConn("alpha", "alpha-svc", 3)
	mike := newBehaviorConn("mike", "mike-svc", 5)

	for _, conn := range []*Connection{zeta, alpha, mike} {
		providers[conn.Method] = &healthStubProvider{name: conn.Method, healthy: true}
	}
	registerBehaviorConns(t, fm, cfg, zeta, alpha, mike)

	if got := fm.findBestBackup(""); got != zeta {
		t.Fatalf("findBestBackup() = %q (priority %d), want zeta (priority %d)",
			got.ID, got.GetPriority(), zeta.GetPriority())
	}

	if got := fm.findBestBackup(zeta.ID); got != alpha {
		t.Fatalf("findBestBackup(excluding zeta) = %q (priority %d), want alpha (priority %d)",
			got.ID, got.GetPriority(), alpha.GetPriority())
	}

	if got := fm.findBestBackup(alpha.ID); got != zeta {
		t.Fatalf("findBestBackup(excluding alpha) = %q (priority %d), want zeta (priority %d)",
			got.ID, got.GetPriority(), zeta.GetPriority())
	}
}

func TestFindBestBackupSamePriorityIsDeterministic(t *testing.T) {
	cfg := behaviorTestConfig()
	providers := map[string]ProviderProvider{
		"bravo":   &healthStubProvider{name: "bravo", healthy: true},
		"alpha":   &healthStubProvider{name: "alpha", healthy: true},
		"charlie": &healthStubProvider{name: "charlie", healthy: true},
	}
	fm := NewFailoverManager(cfg, NewEventPublisher(64), nil, providers, nil)

	bravo := newBehaviorConn("bravo", "bravo", 2)
	alpha := newBehaviorConn("alpha", "alpha", 2)
	charlie := newBehaviorConn("charlie", "charlie", 2)
	registerBehaviorConns(t, fm, cfg, bravo, alpha, charlie)

	if got := fm.findBestBackup(""); got != alpha {
		t.Fatalf("same-priority selection = %q, want alpha (lowest ID wins the tie)", got.ID)
	}

	// Repeat the selection many times: map iteration order is randomized per
	// call, so a nondeterministic tie-break eventually surfaces here.
	for i := 0; i < 250; i++ {
		if got := fm.findBestBackup(""); got != alpha {
			t.Fatalf("iteration %d: same-priority selection flipped to %q; tie-break must be deterministic", i, got.ID)
		}
	}
}

func TestFailureThresholdMarksUnhealthyOnlyAtBoundary(t *testing.T) {
	cfg := behaviorTestConfig()
	provider := &healthStubProvider{name: "stub-svc", healthy: true}
	fm := NewFailoverManager(cfg, NewEventPublisher(16), nil, map[string]ProviderProvider{
		provider.Name(): provider,
	}, nil)
	conn := newBehaviorConn("conn-1", provider.Name(), 0)
	fm.RegisterConnection(conn)
	runSuccessChecks(fm, conn, cfg.RecoveryThreshold)

	snap := snapshotHealth(t, fm, conn.ID)
	if !snap.healthy || snap.successes != cfg.RecoveryThreshold || snap.failures != 0 {
		t.Fatalf("precondition: healthy=%v successes=%d failures=%d, want healthy with %d successes",
			snap.healthy, snap.successes, snap.failures, cfg.RecoveryThreshold)
	}

	subscriber := fm.eventPublisher.Subscribe("all", func(*ConnectionEvent) bool { return true })

	provider.healthy = false

	// One failure below the threshold must not flip the health flag.
	fm.checkConnection(conn)
	snap = snapshotHealth(t, fm, conn.ID)
	if !snap.healthy || snap.failures != 1 || snap.successes != 0 {
		t.Fatalf("after 1 failure: healthy=%v failures=%d successes=%d, want healthy with failures=1 successes=0",
			snap.healthy, snap.failures, snap.successes)
	}

	// Still below the threshold.
	fm.checkConnection(conn)
	snap = snapshotHealth(t, fm, conn.ID)
	if !snap.healthy || snap.failures != 2 {
		t.Fatalf("after 2 failures: healthy=%v failures=%d, want healthy with failures=2",
			snap.healthy, snap.failures)
	}

	// The threshold-th failure flips the flag.
	fm.checkConnection(conn)
	snap = snapshotHealth(t, fm, conn.ID)
	if snap.healthy || snap.failures != cfg.FailureThreshold {
		t.Fatalf("after %d failures: healthy=%v failures=%d, want unhealthy with failures=%d",
			cfg.FailureThreshold, snap.healthy, snap.failures, cfg.FailureThreshold)
	}

	event := waitForEvent(t, subscriber.Channel)
	if event.Type != EventError || event.ConnID != conn.ID {
		t.Fatalf("threshold-crossing event = %v for %s, want EventError for %s",
			event.Type, event.ConnID, conn.ID)
	}
	assertNoEvent(t, subscriber.Channel)
}

func TestRecoveryThresholdMarksHealthyOnlyAtBoundary(t *testing.T) {
	cfg := behaviorTestConfig()
	provider := &healthStubProvider{name: "stub-svc", healthy: false}
	fm := NewFailoverManager(cfg, NewEventPublisher(16), nil, map[string]ProviderProvider{
		provider.Name(): provider,
	}, nil)
	conn := newBehaviorConn("conn-1", provider.Name(), 0)
	fm.RegisterConnection(conn)

	// The provider recovers; successes must accumulate before the connection
	// is trusted again.
	provider.healthy = true

	// RecoveryThreshold-1 successes keep the connection unhealthy.
	for i := 1; i < cfg.RecoveryThreshold; i++ {
		fm.checkConnection(conn)
		snap := snapshotHealth(t, fm, conn.ID)
		if snap.healthy || snap.successes != i || snap.failures != 0 {
			t.Fatalf("after %d successes: healthy=%v successes=%d failures=%d, want unhealthy with successes=%d failures=0",
				i, snap.healthy, snap.successes, snap.failures, i)
		}
	}

	// The threshold-th success flips the flag.
	fm.checkConnection(conn)
	snap := snapshotHealth(t, fm, conn.ID)
	if !snap.healthy || snap.successes != cfg.RecoveryThreshold || snap.failures != 0 {
		t.Fatalf("after %d successes: healthy=%v successes=%d failures=%d, want healthy with successes=%d failures=0",
			cfg.RecoveryThreshold, snap.healthy, snap.successes, snap.failures, cfg.RecoveryThreshold)
	}

	// Staying healthy keeps the flag set as successes keep accumulating.
	fm.checkConnection(conn)
	snap = snapshotHealth(t, fm, conn.ID)
	if !snap.healthy || snap.successes != cfg.RecoveryThreshold+1 {
		t.Fatalf("after %d successes: healthy=%v successes=%d, want healthy with successes=%d",
			cfg.RecoveryThreshold+1, snap.healthy, snap.successes, cfg.RecoveryThreshold+1)
	}
}

func TestFailureStreakResetsOnSuccess(t *testing.T) {
	cfg := behaviorTestConfig()
	provider := &healthStubProvider{name: "stub-svc", healthy: true}
	fm := NewFailoverManager(cfg, NewEventPublisher(16), nil, map[string]ProviderProvider{
		provider.Name(): provider,
	}, nil)
	conn := newBehaviorConn("conn-1", provider.Name(), 0)
	fm.RegisterConnection(conn)
	runSuccessChecks(fm, conn, cfg.RecoveryThreshold)

	// Two failures, then one success: the success must reset the streak so the
	// two earlier failures cannot combine with later ones.
	provider.healthy = false
	fm.checkConnection(conn)
	fm.checkConnection(conn)

	provider.healthy = true
	fm.checkConnection(conn)
	snap := snapshotHealth(t, fm, conn.ID)
	if !snap.healthy || snap.failures != 0 || snap.successes != 1 {
		t.Fatalf("after interleaved success: healthy=%v failures=%d successes=%d, want healthy failures=0 successes=1",
			snap.healthy, snap.failures, snap.successes)
	}

	// Two more failures: still a fresh streak, still below the threshold.
	provider.healthy = false
	fm.checkConnection(conn)
	fm.checkConnection(conn)
	snap = snapshotHealth(t, fm, conn.ID)
	if !snap.healthy || snap.failures != 2 {
		t.Fatalf("after fresh streak of 2: healthy=%v failures=%d, want healthy failures=2",
			snap.healthy, snap.failures)
	}

	// Only the third consecutive failure marks the connection unhealthy.
	fm.checkConnection(conn)
	snap = snapshotHealth(t, fm, conn.ID)
	if snap.healthy || snap.failures != cfg.FailureThreshold {
		t.Fatalf("after third consecutive failure: healthy=%v failures=%d, want unhealthy failures=%d",
			snap.healthy, snap.failures, cfg.FailureThreshold)
	}
}

// TestProviderHealthDrivesFailoverAndRecoveryCycle exercises the full
// documented lifecycle: a healthy pool keeps the primary, failure at the
// failure threshold promotes the backup, and recovery at the recovery
// threshold hands control back. Every outcome is driven solely by
// provider-reported health, with no transport probe or live monitor loop.
func TestProviderHealthDrivesFailoverAndRecoveryCycle(t *testing.T) {
	cfg := behaviorTestConfig()
	primaryProvider := &healthStubProvider{name: "primary-svc", healthy: true}
	backupProvider := &healthStubProvider{name: "backup-svc", healthy: true}
	fm := NewFailoverManager(cfg, NewEventPublisher(64), nil, map[string]ProviderProvider{
		primaryProvider.Name(): primaryProvider,
		backupProvider.Name():  backupProvider,
	}, nil)

	primary := newBehaviorConn("primary", primaryProvider.Name(), 0)
	backup := newBehaviorConn("backup", backupProvider.Name(), 1)
	registerBehaviorConns(t, fm, cfg, primary, backup)
	if err := fm.SetPrimary(primary.ID); err != nil {
		t.Fatalf("SetPrimary(primary) failed: %v", err)
	}

	subscriber := fm.eventPublisher.Subscribe("all", func(*ConnectionEvent) bool { return true })

	// Everything healthy: no switch.
	fm.performHealthChecks()
	if fm.GetPrimary() != primary.ID {
		t.Fatalf("primary = %q with all providers healthy, want %q", fm.GetPrimary(), primary.ID)
	}

	// Primary provider degrades; the failure threshold controls when the
	// backup is promoted.
	primaryProvider.healthy = false
	for i := 1; i <= cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
		if i < cfg.FailureThreshold && fm.GetPrimary() != primary.ID {
			t.Fatalf("primary switched to %q after %d of %d failed checks",
				fm.GetPrimary(), i, cfg.FailureThreshold)
		}
	}
	if fm.GetPrimary() != backup.ID {
		t.Fatalf("after %d failed checks primary = %q, want failover to %q",
			cfg.FailureThreshold, fm.GetPrimary(), backup.ID)
	}
	if primary.IsPrimaryConnection() || !backup.IsPrimaryConnection() {
		t.Fatalf("primary flags after failover: primary=%v backup=%v, want primary=false backup=true",
			primary.IsPrimaryConnection(), backup.IsPrimaryConnection())
	}

	event := waitForEvent(t, subscriber.Channel)
	if event.Type != EventError || event.ConnID != primary.ID {
		t.Fatalf("first threshold event = %v for %s, want EventError for %s",
			event.Type, event.ConnID, primary.ID)
	}
	event = waitForEvent(t, subscriber.Channel)
	if event.Type != EventFailover {
		t.Fatalf("failover event = %v, want EventFailover", event.Type)
	}
	if data, ok := event.Data.(map[string]string); !ok || data["old_primary"] != primary.ID || data["new_primary"] != backup.ID {
		t.Fatalf("failover event data = %#v, want old_primary=%s new_primary=%s",
			event.Data, primary.ID, backup.ID)
	}

	// Primary provider recovers; the recovery threshold controls when control
	// is handed back to the higher-priority connection.
	primaryProvider.healthy = true
	fm.performHealthChecks()
	if fm.GetPrimary() != backup.ID {
		t.Fatalf("primary = %q after 1 of %d recovered checks, want %q until the recovery threshold",
			fm.GetPrimary(), cfg.RecoveryThreshold, backup.ID)
	}
	fm.performHealthChecks()
	if fm.GetPrimary() != primary.ID {
		t.Fatalf("after %d recovered checks primary = %q, want automatic recovery to %q",
			cfg.RecoveryThreshold, fm.GetPrimary(), primary.ID)
	}
	if !primary.IsPrimaryConnection() || backup.IsPrimaryConnection() {
		t.Fatalf("primary flags after recovery: primary=%v backup=%v, want primary=true backup=false",
			primary.IsPrimaryConnection(), backup.IsPrimaryConnection())
	}

	event = waitForEvent(t, subscriber.Channel)
	if event.Type != EventPrimaryChange {
		t.Fatalf("recovery event = %v, want EventPrimaryChange", event.Type)
	}
	if data, ok := event.Data.(map[string]string); !ok || data["new_primary"] != primary.ID {
		t.Fatalf("recovery event data = %#v, want new_primary=%s", event.Data, primary.ID)
	}

	// The cycle repeats: renewed primary failure re-selects the backup.
	primaryProvider.healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
	}
	if fm.GetPrimary() != backup.ID {
		t.Fatalf("on repeated failure primary = %q, want re-selection of %q",
			fm.GetPrimary(), backup.ID)
	}
}

func TestFailoverSkipsUnhealthyHigherPriorityBackup(t *testing.T) {
	cfg := behaviorTestConfig()
	primaryProvider := &healthStubProvider{name: "primary-svc", healthy: true}
	deadProvider := &healthStubProvider{name: "dead-svc", healthy: false}
	goodProvider := &healthStubProvider{name: "good-svc", healthy: true}
	fm := NewFailoverManager(cfg, NewEventPublisher(64), nil, map[string]ProviderProvider{
		primaryProvider.Name(): primaryProvider,
		deadProvider.Name():    deadProvider,
		goodProvider.Name():    goodProvider,
	}, nil)

	primary := newBehaviorConn("primary", primaryProvider.Name(), 0)
	deadBackup := newBehaviorConn("dead-backup", deadProvider.Name(), 1)
	goodBackup := newBehaviorConn("good-backup", goodProvider.Name(), 2)

	// dead-backup must first earn a healthy record through real checks, then
	// lose it through real checks, so the pool reflects genuine health state.
	registerBehaviorConns(t, fm, cfg, primary, deadBackup, goodBackup)
	deadProvider.healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.checkConnection(deadBackup)
	}
	fm.SetPrimary(primary.ID)

	subscriber := fm.eventPublisher.Subscribe("all", func(*ConnectionEvent) bool { return true })

	primaryProvider.healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
	}

	if fm.GetPrimary() != goodBackup.ID {
		t.Fatalf("failover landed on %q, want %q (unhealthy priority-1 backup must be skipped)",
			fm.GetPrimary(), goodBackup.ID)
	}
	if deadBackup.IsPrimaryConnection() {
		t.Fatal("unhealthy backup was promoted to primary")
	}
	if !goodBackup.IsPrimaryConnection() {
		t.Fatal("selected backup was not marked primary")
	}

	// The failover event must name the healthy backup as the new primary.
	var failoverData map[string]string
	for {
		event := waitForEvent(t, subscriber.Channel)
		if event.Type == EventFailover {
			if data, ok := event.Data.(map[string]string); ok {
				failoverData = data
			}
			break
		}
		// Skip the threshold-crossing error events that precede the failover.
		if event.Type != EventError {
			t.Fatalf("unexpected event %v while waiting for failover", event.Type)
		}
	}
	if failoverData == nil || failoverData["new_primary"] != goodBackup.ID {
		t.Fatalf("failover event data = %#v, want new_primary=%s", failoverData, goodBackup.ID)
	}
}

func TestLegacyProviderHealthDrivesFailover(t *testing.T) {
	cfg := behaviorTestConfig()
	primaryProvider := &legacyHealthStubProvider{name: "legacy-primary", healthy: true}
	backupProvider := &legacyHealthStubProvider{name: "legacy-backup", healthy: true}
	fm := NewFailoverManager(cfg, NewEventPublisher(16), nil, map[string]ProviderProvider{
		primaryProvider.Name(): primaryProvider,
		backupProvider.Name():  backupProvider,
	}, nil)

	primary := newBehaviorConn("primary", primaryProvider.Name(), 0)
	backup := newBehaviorConn("backup", backupProvider.Name(), 1)
	registerBehaviorConns(t, fm, cfg, primary, backup)
	if err := fm.SetPrimary(primary.ID); err != nil {
		t.Fatalf("SetPrimary(primary) failed: %v", err)
	}

	// Legacy providers only answer IsHealthy; the failover manager must still
	// promote and recover based on that signal alone.
	primaryProvider.healthy = false
	for i := 0; i < cfg.FailureThreshold; i++ {
		fm.performHealthChecks()
	}
	if fm.GetPrimary() != backup.ID {
		t.Fatalf("legacy-provider failover landed on %q, want %q", fm.GetPrimary(), backup.ID)
	}

	primaryProvider.healthy = true
	for i := 0; i < cfg.RecoveryThreshold; i++ {
		fm.performHealthChecks()
	}
	if fm.GetPrimary() != primary.ID {
		t.Fatalf("legacy-provider recovery landed on %q, want %q", fm.GetPrimary(), primary.ID)
	}
}

func TestAutoRecoverySelectsBestCandidateDeterministically(t *testing.T) {
	cfg := behaviorTestConfig()
	buildPool := func(priorities map[string]int) (*FailoverManager, *Connection) {
		providers := make(map[string]ProviderProvider)
		fm := NewFailoverManager(cfg, NewEventPublisher(64), nil, providers, nil)
		var current *Connection
		for id, priority := range priorities {
			providers[id+"-svc"] = &healthStubProvider{name: id + "-svc", healthy: true}
			conn := newBehaviorConn(id, id+"-svc", priority)
			fm.RegisterConnection(conn)
			runSuccessChecks(fm, conn, cfg.RecoveryThreshold)
			if id == "current" {
				current = conn
			}
		}
		if err := fm.SetPrimary(current.ID); err != nil {
			t.Fatalf("SetPrimary(current) failed: %v", err)
		}
		return fm, current
	}

	// Distinct priorities: recovery must pick the highest-priority healthy
	// candidate even when the map would iterate in any order.
	fm, current := buildPool(map[string]int{
		"gold":    1,
		"silver":  2,
		"bronze":  3,
		"current": 9,
	})
	for i := 0; i < 50; i++ {
		fm.checkForBetterPrimary(current.ID)
		if fm.GetPrimary() != "gold" {
			t.Fatalf("iteration %d: recovery from %q chose %q, want gold", i, current.ID, fm.GetPrimary())
		}
		if err := fm.SetPrimary(current.ID); err != nil {
			t.Fatalf("re-priming primary failed: %v", err)
		}
	}

	// Same-priority ties: the lowest connection ID must win deterministically.
	tieFm, tieCurrent := buildPool(map[string]int{
		"cand-b":  1,
		"cand-a":  1,
		"current": 5,
	})
	for i := 0; i < 50; i++ {
		tieFm.checkForBetterPrimary(tieCurrent.ID)
		if got := tieFm.GetPrimary(); got != "cand-a" {
			t.Fatalf("iteration %d: same-priority recovery chose %q, want cand-a", i, got)
		}
		if err := tieFm.SetPrimary(tieCurrent.ID); err != nil {
			t.Fatalf("re-priming primary failed: %v", err)
		}
	}
}
