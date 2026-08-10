package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ConteMan/muxio/internal/paths"
	"github.com/ConteMan/muxio/internal/store/sqlite"
)

func TestDBPathDoesNotCreateAnything(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv(paths.HomeEnv, home)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "path"}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	want := filepath.Join(home, paths.DatabaseFile)
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("db path created the data directory: %v", err)
	}
}

func TestDBRejectsUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := Run(context.Background(), []string{"db", "nope"}, nil, &stdout, &stderr); code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if code := Run(context.Background(), []string{"db"}, nil, &stdout, &stderr); code != exitUsage {
		t.Fatalf("bare db: exit code = %d, want %d", code, exitUsage)
	}
}

func TestImportRequiresSource(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"import"}, strings.NewReader(""), &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "--source") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// The headline acceptance case: importing the same file twice must not create
// a second copy of anything.
func TestImportIsIdempotentAcrossRuns(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.HomeEnv, home)

	input := strings.Join([]string{
		`{"external_id":"note-1","title":"First","body":"one"}`,
		`{"external_id":"note-2","title":"Second","body":"two"}`,
	}, "\n")

	first := runImportCommand(t, input)
	if first.code != exitOK {
		t.Fatalf("first import exit = %d, stderr = %q", first.code, first.stderr)
	}
	if !strings.Contains(first.stdout, "imported=2 duplicate=0 failed=0") {
		t.Fatalf("first import reported %q", first.stdout)
	}

	second := runImportCommand(t, input)
	if second.code != exitOK {
		t.Fatalf("second import exit = %d, stderr = %q", second.code, second.stderr)
	}
	if !strings.Contains(second.stdout, "imported=0 duplicate=2 failed=0") {
		t.Fatalf("second import reported %q", second.stdout)
	}

	assertCaptureCount(t, home, 2)
}

func TestImportKeepsBothVersionsWhenContentChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.HomeEnv, home)

	runImportCommand(t, `{"external_id":"note-1","title":"Title","body":"original"}`)
	revised := runImportCommand(t, `{"external_id":"note-1","title":"Title","body":"revised"}`)

	if !strings.Contains(revised.stdout, "imported=1 duplicate=0 failed=0") {
		t.Fatalf("revised import reported %q", revised.stdout)
	}
	assertCaptureCount(t, home, 2)
}

func TestImportReportsFailedLinesAndStillCommitsTheRest(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.HomeEnv, home)

	input := strings.Join([]string{
		`{"external_id":"good-1","body":"kept"}`,
		`{"body":"no external id"}`,
		`{"external_id":"good-2","body":"also kept"}`,
	}, "\n")

	result := runImportCommand(t, input)

	if result.code != exitError {
		t.Fatalf("exit code = %d, want %d when a line fails", result.code, exitError)
	}
	if !strings.Contains(result.stdout, "imported=2 duplicate=0 failed=1") {
		t.Fatalf("stdout = %q", result.stdout)
	}
	// The rejected line is reported as structured output, not free text.
	if !strings.Contains(result.stderr, `"msg":"line rejected"`) ||
		!strings.Contains(result.stderr, `"line":2`) {
		t.Fatalf("stderr = %q, want a structured record of the failing line", result.stderr)
	}
	assertCaptureCount(t, home, 2)
}

func TestImportCreatesDataDirectory(t *testing.T) {
	home := filepath.Join(t.TempDir(), "created-on-demand")
	t.Setenv(paths.HomeEnv, home)

	if result := runImportCommand(t, `{"external_id":"a","body":"x"}`); result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}

	if _, err := os.Stat(filepath.Join(home, paths.DatabaseFile)); err != nil {
		t.Fatalf("database was not created: %v", err)
	}
}

type commandResult struct {
	code   int
	stdout string
	stderr string
}

func runImportCommand(t *testing.T, input string) commandResult {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(),
		[]string{"import", "--source", "notes"},
		strings.NewReader(input), &stdout, &stderr)

	return commandResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func assertCaptureCount(t *testing.T, home string, want int) {
	t.Helper()
	ctx := context.Background()

	store, err := sqlite.Open(ctx, filepath.Join(home, paths.DatabaseFile))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = store.Close() }()

	sourceID, err := store.EnsureSource(ctx, "notes", "manual")
	if err != nil {
		t.Fatalf("EnsureSource: %v", err)
	}
	count, err := store.CountCaptures(ctx, sourceID)
	if err != nil {
		t.Fatalf("CountCaptures: %v", err)
	}
	if count != want {
		t.Fatalf("captures = %d, want %d", count, want)
	}
}
