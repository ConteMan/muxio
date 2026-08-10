package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/ConteMan/muxio/internal/store/sqlite"
)

const runsUsage = `Usage:
  muxio runs [--limit N]    List recent runs, newest first
  muxio runs show <id>      Show one run with its events
`

func runRuns(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "show" {
		return runRunsShow(ctx, args[1:], stdout, stderr)
	}

	flags := flag.NewFlagSet("runs", flag.ContinueOnError)
	flags.SetOutput(stderr)
	limit := flags.Int("limit", 20, "how many runs to list")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		_, _ = io.WriteString(stderr, runsUsage)
		return exitUsage
	}

	store, cleanup, err := openStore(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "runs: %v\n", err)
		return exitError
	}
	defer cleanup()

	summaries, err := store.ListRuns(ctx, *limit)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "runs: %v\n", err)
		return exitError
	}
	if len(summaries) == 0 {
		_, _ = fmt.Fprintln(stdout, "no runs recorded yet")
		return exitOK
	}

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "ID\tSOURCE\tSTATUS\tSTARTED\tIMPORTED\tDUPLICATE\tFAILED")
	for _, summary := range summaries {
		_, _ = fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%d\t%d\t%d\n",
			summary.ID, summary.SourceName, summary.Status, summary.StartedAt,
			summary.Counts.Imported, summary.Counts.Duplicate, summary.Counts.Failed)
	}
	if err := table.Flush(); err != nil {
		_, _ = fmt.Fprintf(stderr, "runs: %v\n", err)
		return exitError
	}
	return exitOK
}

func runRunsShow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = io.WriteString(stderr, runsUsage)
		return exitUsage
	}
	runID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || runID <= 0 {
		_, _ = fmt.Fprintf(stderr, "runs show: %q is not a run id\n", args[0])
		return exitUsage
	}

	store, cleanup, err := openStore(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "runs show: %v\n", err)
		return exitError
	}
	defer cleanup()

	summary, events, err := store.GetRun(ctx, runID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "runs show: %v\n", err)
		return exitError
	}

	printRunSummary(stdout, summary)
	printRunEvents(stdout, events)
	return exitOK
}

func printRunSummary(stdout io.Writer, summary sqlite.RunSummary) {
	fields := [][2]string{
		{"source", summary.SourceName},
		{"trigger", summary.Trigger},
		{"status", string(summary.Status)},
		{"started", summary.StartedAt},
		{"finished", orPlaceholder(summary.FinishedAt)},
		{"imported", strconv.Itoa(summary.Counts.Imported)},
		{"duplicate", strconv.Itoa(summary.Counts.Duplicate)},
		{"failed", strconv.Itoa(summary.Counts.Failed)},
	}
	if summary.LastError != "" {
		fields = append(fields, [2]string{"error", summary.LastError})
	}

	_, _ = fmt.Fprintf(stdout, "Run %d\n", summary.ID)
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for _, field := range fields {
		_, _ = fmt.Fprintf(table, "  %s\t%s\n", field[0], field[1])
	}
	_ = table.Flush()
}

func printRunEvents(stdout io.Writer, events []sqlite.EventRecord) {
	if len(events) == 0 {
		_, _ = fmt.Fprintln(stdout, "\nNo events recorded.")
		return
	}

	_, _ = fmt.Fprintf(stdout, "\nEvents (%d)\n", len(events))
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for _, event := range events {
		_, _ = fmt.Fprintf(table, "  %s\t%s\t%s\n",
			event.OccurredAt, event.Level, event.Message)
	}
	_ = table.Flush()
}

func orPlaceholder(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
