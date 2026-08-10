package cli

import (
	"bytes"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer lets a test observe output while the command still writes to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "muxio dev" {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"unknown"}, &stdout, &stderr)

	if code != 64 {
		t.Fatalf("exit code = %d, want 64", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestValidateLoopbackAddress(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"} {
		if err := validateLoopbackAddress(addr); err != nil {
			t.Errorf("validateLoopbackAddress(%q): %v", addr, err)
		}
	}
	if err := validateLoopbackAddress("0.0.0.0:8080"); err == nil {
		t.Fatal("non-loopback address was accepted")
	}
}

func TestRequireLoopbackAddr(t *testing.T) {
	if err := requireLoopbackAddr(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}); err != nil {
		t.Errorf("loopback addr rejected: %v", err)
	}
	if err := requireLoopbackAddr(&net.TCPAddr{IP: net.IPv4(192, 168, 1, 10), Port: 8080}); err == nil {
		t.Error("non-loopback addr accepted")
	}
	if err := requireLoopbackAddr(&net.UnixAddr{Name: "/tmp/muxio.sock"}); err == nil {
		t.Error("non-TCP addr accepted")
	}
}

func TestServeRejectsNonLoopbackBind(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run(context.Background(), []string{"serve", "--addr", "0.0.0.0:0"}, &stdout, &stderr)

	if code != 64 {
		t.Fatalf("exit code = %d, want 64", code)
	}
	if !strings.Contains(stderr.String(), "loopback") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestServeShutsDownOnContextCancel(t *testing.T) {
	var stdout, stderr syncBuffer
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, []string{"serve", "--addr", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	// Wait for the listener to report its bound address before cancelling.
	deadline := time.After(5 * time.Second)
	for !strings.Contains(stdout.String(), "listening on") {
		select {
		case <-deadline:
			t.Fatal("server did not report a listen address")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("serve did not shut down after context cancel")
	}
}
