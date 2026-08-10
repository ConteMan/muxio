package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ConteMan/muxio/internal/paths"
)

func TestRunsListsRecordedRuns(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	runImportCommand(t, `{"external_id":"note-1","body":"one"}`)
	runImportCommand(t, `{"external_id":"note-2","body":"two"}`)

	result := runCommand(t, "runs")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}

	if !strings.Contains(result.stdout, "IMPORTED") {
		t.Fatalf("no header in output: %q", result.stdout)
	}
	if occurrences := strings.Count(result.stdout, "succeeded"); occurrences != 2 {
		t.Fatalf("found %d succeeded runs, want 2:\n%s", occurrences, result.stdout)
	}
	if !strings.Contains(result.stdout, "notes") {
		t.Fatalf("source name missing: %q", result.stdout)
	}
}

func TestRunsReportsEmptyHistory(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	result := runCommand(t, "runs")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}
	if !strings.Contains(result.stdout, "no runs recorded yet") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

// The whole point of the run log: after the process exits, you can still find
// out which line failed and why.
func TestRunsShowExplainsRejectedLines(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	imported := runImportCommand(t, strings.Join([]string{
		`{"external_id":"good","body":"kept"}`,
		`{"body":"no external id"}`,
	}, "\n"))
	if imported.code != exitError {
		t.Fatalf("import exit = %d, want %d", imported.code, exitError)
	}
	// The failure message must tell the user where to look.
	if !strings.Contains(imported.stderr, "muxio runs show") {
		t.Fatalf("import stderr = %q", imported.stderr)
	}

	result := runCommand(t, "runs", "show", "1")
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}

	for _, want := range []string{
		"succeeded",       // rejected lines do not fail the run
		"failed",          // the count is present
		"line 2 rejected", // and the specific line is explained
		"external_id is required",
		"import started",
		"import finished",
	} {
		if !strings.Contains(result.stdout, want) {
			t.Errorf("output missing %q:\n%s", want, result.stdout)
		}
	}
}

func TestRunsShowRejectsBadArguments(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	for _, args := range [][]string{
		{"runs", "show"},
		{"runs", "show", "abc"},
		{"runs", "show", "0"},
		{"runs", "show", "1", "2"},
	} {
		if result := runCommand(t, args...); result.code != exitUsage {
			t.Errorf("%v: exit = %d, want %d", args, result.code, exitUsage)
		}
	}
}

func TestRunsShowReportsMissingRun(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	result := runCommand(t, "runs", "show", "999")
	if result.code != exitError {
		t.Fatalf("exit = %d, want %d", result.code, exitError)
	}
	if !strings.Contains(result.stderr, "run not found") {
		t.Fatalf("stderr = %q", result.stderr)
	}
}

func TestImportRejectsUnknownLogLevel(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(),
		[]string{"import", "--source", "notes", "--log-level", "loud"},
		strings.NewReader(""), &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "unknown log level") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// Structured logs carry the run id so terminal output can be matched against
// stored history.
func TestImportLogsAreStructuredAndCarryRunID(t *testing.T) {
	t.Setenv(paths.HomeEnv, t.TempDir())

	result := runImportCommand(t, `{"external_id":"note-1","body":"one"}`)
	if result.code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", result.code, result.stderr)
	}

	if !strings.Contains(result.stderr, `"msg":"import started"`) {
		t.Errorf("stderr is not structured JSON: %q", result.stderr)
	}
	if !strings.Contains(result.stderr, `"run_id":1`) {
		t.Errorf("stderr does not carry the run id: %q", result.stderr)
	}
	if !strings.Contains(result.stdout, "run=1") {
		t.Errorf("stdout does not report the run id: %q", result.stdout)
	}
}

func runCommand(t *testing.T, args ...string) commandResult {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return commandResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}
