package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ConteMan/muxio/internal/paths"
)

func TestConfigPathDoesNotCreateAnything(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv(paths.HomeEnv, home)

	result := runCommand(t, "config", "path")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}
	want := filepath.Join(home, paths.ConfigFileName)
	if got := strings.TrimSpace(result.stdout); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("config path created the directory: %v", err)
	}
}

// Without a config file everything still works and reports the defaults.
func TestConfigShowWithoutFile(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	result := runCommand(t, "config", "show")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}
	if !strings.Contains(result.stdout, "not created, using defaults") {
		t.Errorf("stdout does not say the file is absent:\n%s", result.stdout)
	}
	for _, want := range []string{"server.addr", "127.0.0.1:8080", "default"} {
		if !strings.Contains(result.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, result.stdout)
		}
	}
}

func TestConfigInitWritesCommentedDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.HomeEnv, home)

	result := runCommand(t, "config", "init")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}

	configPath := filepath.Join(home, paths.ConfigFileName)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), "# Muxio configuration") {
		t.Errorf("generated file has no comments:\n%s", content)
	}
	if !strings.Contains(string(content), "[server]") {
		t.Errorf("generated file has no sections:\n%s", content)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %04o, want 0600", perm)
	}

	// A second init must not silently discard an existing configuration.
	if second := runCommand(t, "config", "init"); second.code != exitError {
		t.Fatalf("second init exit = %d, want %d", second.code, exitError)
	}
}

func TestConfigSetPersistsAndKeepsComments(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.HomeEnv, home)

	runCommand(t, "config", "init")
	result := runCommand(t, "config", "set", "retention.run_event_days", "7")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}

	content, err := os.ReadFile(filepath.Join(home, paths.ConfigFileName))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), "run_event_days = 7") {
		t.Errorf("value was not persisted:\n%s", content)
	}
	// Generated comments are re-rendered, so they survive a write.
	if !strings.Contains(string(content), "# Muxio configuration") {
		t.Errorf("comments were lost on write:\n%s", content)
	}

	shown := runCommand(t, "config", "show")
	if !strings.Contains(shown.stdout, "file") {
		t.Errorf("config show does not report the file origin:\n%s", shown.stdout)
	}
}

func TestConfigSetRejectsInvalidValue(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())
	runCommand(t, "config", "init")

	result := runCommand(t, "config", "set", "server.addr", "0.0.0.0:8080")
	if result.code != exitUsage {
		t.Fatalf("exit = %d, want %d", result.code, exitUsage)
	}
	if !strings.Contains(result.stderr, "loopback") {
		t.Fatalf("stderr = %q", result.stderr)
	}
}

func TestConfigSetRejectsUnknownKey(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	result := runCommand(t, "config", "set", "server.port", "9090")
	if result.code != exitUsage {
		t.Fatalf("exit = %d, want %d", result.code, exitUsage)
	}
	if !strings.Contains(result.stderr, "unknown setting") {
		t.Fatalf("stderr = %q", result.stderr)
	}
}

func TestConfigRejectsBadSubcommands(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	for _, args := range [][]string{
		{"config"},
		{"config", "nope"},
		{"config", "path", "extra"},
		{"config", "set", "only-one-arg"},
	} {
		if result := runCommand(t, args...); result.code != exitUsage {
			t.Errorf("%v: exit = %d, want %d", args, result.code, exitUsage)
		}
	}
}

// An environment override must not be baked into the file by a later write.
func TestConfigSetDoesNotPersistEnvironmentOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.HomeEnv, home)
	t.Setenv("MUXIO_LOG_LEVEL", "error")

	runCommand(t, "config", "init")
	result := runCommand(t, "config", "set", "retention.run_event_days", "9")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}

	content, err := os.ReadFile(filepath.Join(home, paths.ConfigFileName))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), `level = "info"`) {
		t.Errorf("the environment override was written into the file:\n%s", content)
	}
}

// The whole point of showing origins: knowing why a value is what it is.
func TestConfigShowReportsEnvironmentOrigin(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())
	t.Setenv("MUXIO_LOG_LEVEL", "debug")

	result := runCommand(t, "config", "show")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}

	var levelLine string
	for _, line := range strings.Split(result.stdout, "\n") {
		if strings.HasPrefix(line, "log.level") {
			levelLine = line
		}
	}
	if !strings.Contains(levelLine, "debug") || !strings.Contains(levelLine, "env") {
		t.Fatalf("log.level line = %q, want the value and an env origin", levelLine)
	}
}

// Configuration must actually reach the running command.
func TestConfiguredLogLevelAffectsImport(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.HomeEnv, home)

	runCommand(t, "config", "init")
	if result := runCommand(t, "config", "set", "log.level", "error"); result.code != exitOK {
		t.Fatalf("config set: %s", result.stderr)
	}

	result := runImportCommand(t, `{"external_id":"note-1","body":"one"}`)
	if result.code != exitOK {
		t.Fatalf("import exit = %d, stderr = %q", result.code, result.stderr)
	}
	// At error level the informational lifecycle logs must not appear.
	if strings.Contains(result.stderr, "import started") {
		t.Fatalf("info logs were emitted at error level:\n%s", result.stderr)
	}
}

func TestConfiguredBodyLimitAffectsImport(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.HomeEnv, home)

	runCommand(t, "config", "init")
	if result := runCommand(t, "config", "set", "capture.max_body_bytes", "16"); result.code != exitOK {
		t.Fatalf("config set: %s", result.stderr)
	}

	result := runImportCommand(t, `{"external_id":"note-1","body":"this body is definitely longer than sixteen bytes"}`)
	if result.code != exitError {
		t.Fatalf("exit = %d, want the oversized body rejected", result.code)
	}
	if !strings.Contains(result.stdout, "failed=1") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}
