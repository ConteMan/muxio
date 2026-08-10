package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

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
