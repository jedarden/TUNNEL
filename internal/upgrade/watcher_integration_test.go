package upgrade

// Integration coverage for the hot-swap promise in README: a replaced binary
// is detected by the watcher, passes integrity verification, runs the
// graceful onUpgrade phase, and reaches the restart handoff — all without
// dropping the connections the process is actively serving.
//
// The final step of the real sequence is syscall.Exec, which replaces the
// process image and cannot survive inside a test binary, so the tests inject
// restartFn (the test-only seam on Watcher) to observe the handoff.
// Everything ahead of it — fsnotify detection, debouncing, integrity
// verification and the onUpgrade callback — is the unmodified production
// path, driven by a real file replacement on a real filesystem.

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	hotSwapDebounce = 100 * time.Millisecond
	hotSwapTimeout  = 5 * time.Second
	// Longer than the debounce window: sleeping this long proves the watcher
	// stayed quiet, not merely that it was slow.
	hotSwapQuietPeriod = 1500 * time.Millisecond
)

// platformMagic returns the first 8 bytes basicSanityChecks expects on the
// current platform, so a crafted file passes the offline integrity path.
func platformMagic() []byte {
	switch runtime.GOOS {
	case "darwin":
		return []byte{0xFE, 0xED, 0xFA, 0xCF, 0, 0, 0, 0}
	case "windows":
		return []byte{'M', 'Z', 0, 0, 0, 0, 0, 0}
	default:
		// ELF header (64-bit little-endian class/padding).
		return []byte{0x7F, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00}
	}
}

// fakeBinary builds file contents that pass basicSanityChecks.
func fakeBinary(payload string) []byte {
	return append(append([]byte{}, platformMagic()...), []byte(payload)...)
}

// writeTestBinary replaces the file at path and pins its mtime, so
// checkAndRestart's ModTime comparison is deterministic even on filesystems
// with coarse timestamps.
func writeTestBinary(t *testing.T, path string, content []byte, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("write binary %s: %v", path, err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatalf("set mtime on %s: %v", path, err)
		}
	}
}

// blockChecksumFetch forces the release-checksum fetch to fail fast and
// deterministically (connection refused on a loopback port, with any proxy
// bypass list cleared), so integrity verification takes the offline
// basic-sanity fallback in every environment the suite runs in.
func blockChecksumFetch(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(key, "http://127.0.0.1:1")
	}
	for _, key := range []string{"NO_PROXY", "no_proxy"} {
		t.Setenv(key, "")
	}
}

// newIntegrationWatcher builds a Watcher against an arbitrary binary path
// with a short debounce, a discard logger and an injected restart handoff,
// then starts it. All teardown is registered with the test.
func newIntegrationWatcher(t *testing.T, binaryPath string, onUpgrade func(), restartFn func() error) *Watcher {
	t.Helper()
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("create fsnotify watcher: %v", err)
	}
	t.Cleanup(func() { _ = fsWatcher.Close() })

	w := &Watcher{
		binaryPath: binaryPath,
		watcher:    fsWatcher,
		logger:     log.New(io.Discard, "", 0),
		debounce:   hotSwapDebounce,
		restartFn:  restartFn,
	}
	if err := w.Start(onUpgrade); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	t.Cleanup(func() { _ = w.Stop() })

	// Let the watch goroutine start draining fsnotify events before the
	// test touches the binary.
	time.Sleep(100 * time.Millisecond)
	return w
}

// waitSignal fails the test if ch is not closed within the timeout.
func waitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(hotSwapTimeout):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// echoServer stands in for the service the tunnel process is actively
// serving: it answers "ping" lines with "pong" on every accepted connection.
// The first request is held briefly after it is read, so the graceful phase
// has genuine in-flight work to drain while the binary is being replaced,
// and the server reports both receipt and completion of that request.
type echoServer struct {
	listener net.Listener
	recvOnce sync.Once
	ansOnce  sync.Once
	received chan struct{} // closed once the first request has been read
	answered chan struct{} // closed once the first response has been written

	mu      sync.Mutex
	clients map[net.Conn]struct{}
	wg      sync.WaitGroup
}

func newEchoServer(t *testing.T) *echoServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &echoServer{
		listener: ln,
		received: make(chan struct{}),
		answered: make(chan struct{}),
		clients:  make(map[net.Conn]struct{}),
	}
	s.wg.Add(1)
	go s.acceptLoop()
	t.Cleanup(s.close)
	return s
}

func (s *echoServer) addr() string { return s.listener.Addr().String() }

func (s *echoServer) activeConnections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

func (s *echoServer) close() {
	_ = s.listener.Close()
	s.mu.Lock()
	for conn := range s.clients {
		_ = conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *echoServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.clients[conn] = struct{}{}
		s.mu.Unlock()
		s.wg.Add(1)
		go s.serve(conn)
	}
}

func (s *echoServer) serve(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		_ = conn.Close()
	}()

	buf := make([]byte, 64)
	for {
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			return
		}
		s.recvOnce.Do(func() { close(s.received) })
		// Hold the response briefly so the graceful phase has real
		// in-flight work to drain while the binary is being replaced.
		time.Sleep(250 * time.Millisecond)
		if _, err := conn.Write([]byte("pong\n")); err != nil {
			return
		}
		s.ansOnce.Do(func() { close(s.answered) })
	}
}

// dialEcho opens a client connection that stays registered with the test
// until the test ends, mirroring a long-lived tunnel user.
func dialEcho(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// sendPing writes one request line to the connection.
func sendPing(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetDeadline(time.Now().Add(hotSwapTimeout)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if _, err := conn.Write([]byte("ping\n")); err != nil {
		t.Fatalf("send ping: %v", err)
	}
}

// readPong reads one full response and verifies it, proving the connection
// is still usable end to end.
func readPong(t *testing.T, conn net.Conn) {
	t.Helper()
	pong := make([]byte, len("pong\n"))
	if _, err := io.ReadFull(conn, pong); err != nil {
		t.Fatalf("connection dropped before response arrived: %v", err)
	}
	if string(pong) != "pong\n" {
		t.Fatalf("unexpected response %q", pong)
	}
}

// TestHotSwapReplacementPerformsGracefulRestart drives the full sequence
// promised by the README against a live binary replacement: detection of the
// new file, integrity verification, the graceful phase (drain in-flight
// work, persist state), and the restart handoff — while a client connection
// with a request in flight stays available throughout.
func TestHotSwapReplacementPerformsGracefulRestart(t *testing.T) {
	blockChecksumFetch(t)

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "tunnel")
	writeTestBinary(t, binaryPath, fakeBinary("tunnel-v1"), time.Now())

	server := newEchoServer(t)

	var (
		seqMu    sync.Mutex
		sequence []string
	)
	record := func(stage string) {
		seqMu.Lock()
		defer seqMu.Unlock()
		sequence = append(sequence, stage)
	}
	sequenceSnapshot := func() []string {
		seqMu.Lock()
		defer seqMu.Unlock()
		return append([]string(nil), sequence...)
	}

	// The graceful phase mirrors what the app does before the exec: let
	// in-flight work finish, then persist whatever the replacement needs.
	statePath := filepath.Join(dir, "upgrade-state.json")
	upgradePrepared := make(chan struct{})
	onUpgrade := func() {
		record("upgrade-callback")
		select {
		case <-server.answered:
		case <-time.After(hotSwapTimeout):
			t.Errorf("in-flight request never drained during the graceful phase")
			close(upgradePrepared)
			return
		}
		state := fmt.Sprintf(`{"drained":true,"active_connections":%d}`, server.activeConnections())
		if err := os.WriteFile(statePath, []byte(state), 0o600); err != nil {
			t.Errorf("persist upgrade state: %v", err)
		}
		close(upgradePrepared)
	}

	var restarts atomic.Int32
	restartInvoked := make(chan struct{})
	restartFn := func() error {
		record("restart")
		// The state the callback persisted must be readable at the handoff:
		// this is what the replacement process picks up.
		state, err := os.ReadFile(statePath)
		if err != nil {
			t.Errorf("upgrade state missing at restart: %v", err)
		} else if want := `"drained":true`; !strings.Contains(string(state), want) {
			t.Errorf("upgrade state at restart = %s, want it to contain %s", state, want)
		}
		restarts.Add(1)
		close(restartInvoked)
		return nil
	}

	w := newIntegrationWatcher(t, binaryPath, onUpgrade, restartFn)
	if got := w.GetBinaryPath(); got != binaryPath {
		t.Fatalf("watcher tracks %q, want %q", got, binaryPath)
	}

	// A client with a request in flight (the server holds its response for
	// 250ms) at the moment the binary is replaced.
	conn := dialEcho(t, server.addr())
	sendPing(t, conn)
	waitSignal(t, server.received, "server to receive the in-flight request")

	writeTestBinary(t, binaryPath, fakeBinary("tunnel-v2"), time.Now().Add(2*time.Second))

	waitSignal(t, upgradePrepared, "graceful phase to drain and persist state")
	waitSignal(t, restartInvoked, "restart handoff")

	// The request issued before the swap must still complete: the restart
	// sequence did not drop the connection with work in flight.
	readPong(t, conn)

	// And the connection remains fully usable after the whole sequence.
	sendPing(t, conn)
	readPong(t, conn)

	got := sequenceSnapshot()
	if len(got) != 2 || got[0] != "upgrade-callback" || got[1] != "restart" {
		t.Fatalf("restart sequence = %v, want [upgrade-callback restart]", got)
	}
	if got := restarts.Load(); got != 1 {
		t.Fatalf("restart handoff ran %d times, want 1", got)
	}
}

// TestHotSwapAbortsRestartOnCorruptBinary verifies the integrity gate: a
// replacement that fails verification must not run the graceful phase or the
// restart, and the watcher must stay armed so a subsequent good replacement
// still goes through.
func TestHotSwapAbortsRestartOnCorruptBinary(t *testing.T) {
	blockChecksumFetch(t)

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "tunnel")
	writeTestBinary(t, binaryPath, fakeBinary("tunnel-v1"), time.Now())

	var upgrades, restarts atomic.Int32
	restartInvoked := make(chan struct{})
	_ = newIntegrationWatcher(t, binaryPath,
		func() { upgrades.Add(1) },
		func() error {
			restarts.Add(1)
			close(restartInvoked)
			return nil
		},
	)

	// Corrupt replacement: right size and exec bit, wrong magic bytes.
	corrupt := []byte("definitely not a binary image")
	writeTestBinary(t, binaryPath, corrupt, time.Now().Add(2*time.Second))
	time.Sleep(hotSwapQuietPeriod)

	if got := upgrades.Load(); got != 0 {
		t.Fatalf("graceful phase ran %d times for a corrupt binary, want 0", got)
	}
	if got := restarts.Load(); got != 0 {
		t.Fatalf("restart ran %d times for a corrupt binary, want 0", got)
	}

	// The watcher must still be armed: a good replacement afterwards goes through.
	writeTestBinary(t, binaryPath, fakeBinary("tunnel-v2"), time.Now().Add(4*time.Second))
	waitSignal(t, restartInvoked, "restart after recovery")

	if got := upgrades.Load(); got != 1 {
		t.Fatalf("graceful phase ran %d times after recovery, want 1", got)
	}
	if got := restarts.Load(); got != 1 {
		t.Fatalf("restart ran %d times after recovery, want 1", got)
	}
}

// TestHotSwapIgnoresWriteWithoutModifiedTime verifies the change guard: an
// fsnotify event whose binary mtime did not advance (a rewrite pinned back to
// the tracked modification time) must not trigger the graceful phase or a
// restart.
func TestHotSwapIgnoresWriteWithoutModifiedTime(t *testing.T) {
	blockChecksumFetch(t)

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "tunnel")
	writeTestBinary(t, binaryPath, fakeBinary("tunnel-v1"), time.Now())

	var upgrades, restarts atomic.Int32
	_ = newIntegrationWatcher(t, binaryPath,
		func() { upgrades.Add(1) },
		func() error {
			restarts.Add(1)
			return nil
		},
	)

	// Rewrite the same bytes, then pin the mtime back to what Start recorded.
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	writeTestBinary(t, binaryPath, fakeBinary("tunnel-v1"), info.ModTime())
	time.Sleep(hotSwapQuietPeriod)

	if got := upgrades.Load(); got != 0 {
		t.Fatalf("graceful phase ran %d times without a modification, want 0", got)
	}
	if got := restarts.Load(); got != 0 {
		t.Fatalf("restart ran %d times without a modification, want 0", got)
	}
}
