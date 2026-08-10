package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("the built-in configuration is invalid: %v", err)
	}
}

// An unconfigured Muxio must run, and must not create a file behind the user's back.
func TestLoadWithoutFileUsesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	loaded, err := Load(path, emptyEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Exists {
		t.Fatal("Load reported a file that does not exist")
	}
	if loaded.Config != Default() {
		t.Fatalf("config = %+v, want defaults", loaded.Config)
	}
	for _, key := range Keys() {
		if origin := loaded.Origin(key); origin != FromDefault {
			t.Errorf("%s came from %s, want default", key, origin)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Load created the file: %v", err)
	}
}

func TestFileValuesOverrideDefaults(t *testing.T) {
	path := writeConfig(t, `
[server]
addr = "127.0.0.1:9999"

[log]
level = "debug"
`)

	loaded, err := Load(path, emptyEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Config.Server.Addr != "127.0.0.1:9999" {
		t.Errorf("addr = %q", loaded.Config.Server.Addr)
	}
	if loaded.Config.Log.Level != "debug" {
		t.Errorf("level = %q", loaded.Config.Log.Level)
	}
	if loaded.Origin("server.addr") != FromFile {
		t.Errorf("addr origin = %s, want file", loaded.Origin("server.addr"))
	}
	// Untouched fields keep their defaults and say so.
	if loaded.Config.Capture.MaxBodyBytes != DefaultMaxBodyBytes {
		t.Errorf("max_body_bytes = %d", loaded.Config.Capture.MaxBodyBytes)
	}
	if loaded.Origin("capture.max_body_bytes") != FromDefault {
		t.Error("an untouched field did not report the default origin")
	}
}

func TestEnvironmentBeatsFile(t *testing.T) {
	path := writeConfig(t, "[log]\nlevel = \"debug\"\n")

	loaded, err := Load(path, envWith(map[string]string{"MUXIO_LOG_LEVEL": "warn"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Config.Log.Level != "warn" {
		t.Fatalf("level = %q, want the environment value", loaded.Config.Log.Level)
	}
	if loaded.Origin("log.level") != FromEnv {
		t.Fatalf("origin = %s, want env", loaded.Origin("log.level"))
	}
}

func TestFlagBeatsEverything(t *testing.T) {
	path := writeConfig(t, "[log]\nlevel = \"debug\"\n")

	loaded, err := Load(path, envWith(map[string]string{"MUXIO_LOG_LEVEL": "warn"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := loaded.Override("log.level", "error"); err != nil {
		t.Fatalf("Override: %v", err)
	}
	if loaded.Config.Log.Level != "error" || loaded.Origin("log.level") != FromFlag {
		t.Fatalf("level = %q from %s", loaded.Config.Log.Level, loaded.Origin("log.level"))
	}
}

// A misspelled key would otherwise leave the user believing a setting applied.
func TestUnknownKeyIsRejected(t *testing.T) {
	path := writeConfig(t, "[server]\nadress = \"127.0.0.1:8080\"\n")

	_, err := Load(path, emptyEnv)
	if err == nil {
		t.Fatal("an unknown key was accepted")
	}
	if !strings.Contains(err.Error(), "adress") {
		t.Fatalf("err = %v, want the offending key", err)
	}
}

func TestInvalidValueReportsFieldAndLine(t *testing.T) {
	path := writeConfig(t, `# comment line
[server]
addr = "0.0.0.0:8080"
`)

	_, err := Load(path, emptyEnv)
	if err == nil {
		t.Fatal("a non-loopback address was accepted")
	}

	var invalid *ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want a ValidationError", err)
	}
	if invalid.Key != "server.addr" {
		t.Errorf("Key = %q", invalid.Key)
	}
	if invalid.Line != 3 {
		t.Errorf("Line = %d, want 3", invalid.Line)
	}
	if !strings.Contains(invalid.Error(), "loopback") {
		t.Errorf("message = %q", invalid.Error())
	}
}

func TestMalformedTOMLReportsPosition(t *testing.T) {
	path := writeConfig(t, "[server\naddr = \"127.0.0.1:8080\"\n")

	_, err := Load(path, emptyEnv)
	if err == nil {
		t.Fatal("malformed TOML was accepted")
	}
	if !strings.Contains(err.Error(), "not valid TOML") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidationBounds(t *testing.T) {
	tests := map[string]string{
		"[capture]\nmax_body_bytes = 0\n":           "capture.max_body_bytes",
		"[capture]\nmax_body_bytes = 99999999999\n": "capture.max_body_bytes",
		"[retention]\nrun_event_days = 0\n":         "retention.run_event_days",
		"[log]\nlevel = \"loud\"\n":                 "log.level",
		"[server]\naddr = \"127.0.0.1\"\n":          "server.addr",
	}

	for content, wantKey := range tests {
		t.Run(wantKey+"/"+strings.TrimSpace(strings.SplitN(content, "\n", 2)[1]), func(t *testing.T) {
			if _, err := Load(writeConfig(t, content), emptyEnv); err == nil {
				t.Fatal("invalid value was accepted")
			} else if !strings.Contains(err.Error(), wantKey) {
				t.Fatalf("err = %v, want it to name %s", err, wantKey)
			}
		})
	}
}

// Writing is a full re-render, so the generated comments must come back.
func TestRenderRoundTrips(t *testing.T) {
	original := Config{
		Server:    Server{Addr: "127.0.0.1:9090"},
		Log:       Log{Level: "warn"},
		Capture:   Capture{MaxBodyBytes: 1024},
		Retention: Retention{RunEventDays: 7},
	}

	path := writeConfig(t, Render(original))
	loaded, err := Load(path, emptyEnv)
	if err != nil {
		t.Fatalf("Load rendered config: %v", err)
	}
	if loaded.Config != original {
		t.Fatalf("round trip = %+v, want %+v", loaded.Config, original)
	}
	if !strings.Contains(Render(original), "# Muxio configuration") {
		t.Error("rendered file lost its generated comments")
	}
	// Every field must appear, or a setting would silently vanish on write.
	for _, key := range Keys() {
		name := key[strings.Index(key, ".")+1:]
		if !strings.Contains(Render(original), name+" = ") {
			t.Errorf("rendered file is missing %s", key)
		}
	}
}

func TestCreateRefusesToClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	if err := Create(path, Default()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != filePerm {
		t.Errorf("permissions = %04o, want %04o", perm, filePerm)
	}

	if err := Create(path, Default()); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
}

// The point of the modification-time check: a settings UI must not silently
// overwrite an edit someone made by hand in the meantime.
func TestWriteDetectsConcurrentEdit(t *testing.T) {
	path := writeConfig(t, Render(Default()))

	loaded, err := Load(path, emptyEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Someone else edits the file after we read it.
	later := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(path, []byte(Render(Default())), filePerm); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	updated, err := Set(loaded.Config, "log.level", "debug")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := Write(path, updated, loaded.ModTime); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}

	// The other edit survived.
	reloaded, err := Load(path, emptyEnv)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Config.Log.Level != DefaultLogLevel {
		t.Fatalf("level = %q, the concurrent edit was overwritten", reloaded.Config.Log.Level)
	}
}

func TestWriteSucceedsWhenUnchanged(t *testing.T) {
	path := writeConfig(t, Render(Default()))

	loaded, err := Load(path, emptyEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	updated, err := Set(loaded.Config, "retention.run_event_days", "7")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := Write(path, updated, loaded.ModTime); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reloaded, err := Load(path, emptyEnv)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Config.Retention.RunEventDays != 7 {
		t.Fatalf("run_event_days = %d", reloaded.Config.Retention.RunEventDays)
	}
}

func TestWriteRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	broken := Default()
	broken.Server.Addr = "0.0.0.0:8080"

	if err := Write(path, broken, time.Time{}); err == nil {
		t.Fatal("an invalid configuration was written")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a rejected write still created the file")
	}
}

func TestSetRejectsUnknownKey(t *testing.T) {
	if _, err := Set(Default(), "server.port", "8080"); err == nil {
		t.Fatal("an unknown key was accepted")
	}
}

func TestRunEventRetention(t *testing.T) {
	c := Default()
	c.Retention.RunEventDays = 3
	if got := c.RunEventRetention(); got != 72*time.Hour {
		t.Fatalf("RunEventRetention() = %v, want 72h", got)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), filePerm); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func emptyEnv(string) (string, bool) { return "", false }

func envWith(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
