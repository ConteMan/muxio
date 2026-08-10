package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	valid := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		" warn ":  slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for name, want := range valid {
		got, err := ParseLevel(name)
		if err != nil {
			t.Errorf("ParseLevel(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", name, got, want)
		}
	}

	if _, err := ParseLevel("loud"); err == nil {
		t.Error("an unknown level was accepted")
	}
}

func TestNewWritesJSON(t *testing.T) {
	var out bytes.Buffer
	New(&out, slog.LevelInfo).Info("import started", "run_id", 7)

	var entry map[string]any
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out.String())
	}
	if entry["msg"] != "import started" {
		t.Errorf("msg = %v", entry["msg"])
	}
	if entry["run_id"] != float64(7) {
		t.Errorf("run_id = %v", entry["run_id"])
	}
}

func TestLevelFiltersOutput(t *testing.T) {
	var out bytes.Buffer
	logger := New(&out, slog.LevelWarn)

	logger.Info("not written")
	if out.Len() != 0 {
		t.Fatalf("info was written at warn level: %q", out.String())
	}

	logger.Warn("written")
	if out.Len() == 0 {
		t.Fatal("warn was filtered out at warn level")
	}
}
