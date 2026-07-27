package common

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

// freePort asks the OS for an unused port so parallel runs don't collide.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("releasing the reserved port: %v", err)
	}
	return port
}

// main registers SIGINT/SIGTERM with signal.NotifyContext, which switches off
// Go's default terminate-on-signal. Serve is what has to act on the cancelled
// context instead — if it doesn't, `docker stop` hangs until the daemon sends
// SIGKILL and running agent subprocesses die mid-stage.
func TestServer_Serve_ReturnsOnContextCancel(t *testing.T) {
	server := NewServer(&Config{ListenPort: freePort(t)}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan error, 1)
	go func() { returned <- server.Serve(ctx) }()

	// Give the listener a moment to come up, so cancelling exercises the
	// shutdown path rather than racing the goroutine start.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("Serve returned an error on clean shutdown: %v", err)
		}
	case <-time.After(shutdownGrace + 2*time.Second):
		t.Fatal("Serve did not return after its context was cancelled")
	}
}

// A port already in use must surface as an error rather than a silent no-op:
// the old implementation logged and fell through, so the process stayed alive
// serving nothing.
func TestServer_Serve_ReportsListenFailure(t *testing.T) {
	port := freePort(t)
	blocker, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("occupying the port: %v", err)
	}
	defer blocker.Close()

	server := NewServer(&Config{ListenPort: port}, nil, nil)

	returned := make(chan error, 1)
	go func() { returned <- server.Serve(context.Background()) }()

	select {
	case err := <-returned:
		if err == nil {
			t.Fatal("Serve returned nil for a port it could not bind")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve blocked instead of reporting the bind failure")
	}
}
