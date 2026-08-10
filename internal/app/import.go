// Package app holds use cases. Adapters such as the CLI and the HTTP API call
// into it; it never depends on them.
package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/ConteMan/muxio/internal/record"
	"github.com/ConteMan/muxio/internal/run"
)

// ManualConnectorKind marks sources created by hand rather than by a connector.
const ManualConnectorKind = "manual"

// maxLineBytes bounds one JSONL line. A 5 MiB body can expand under JSON
// escaping, so the line budget is larger than the body limit.
const maxLineBytes = 32 << 20

// Options carries the resolved settings an import needs. They arrive from
// configuration rather than package constants so a change in config.toml takes
// effect without a rebuild.
type Options struct {
	MaxBodyBytes   int
	EventRetention time.Duration
}

// RunStore is the storage capability an import needs.
type RunStore interface {
	EnsureSource(ctx context.Context, name, connectorKind string) (int64, error)
	StartRun(ctx context.Context, sourceID int64, trigger string) (int64, error)
	Heartbeat(ctx context.Context, runID int64) error
	FinishRun(ctx context.Context, runID int64, status run.Status, lastError string) error
	AddCapture(ctx context.Context, sourceID, runID int64, rec record.Record) (bool, error)
	AppendEvent(ctx context.Context, runID int64, event run.Event) error
	RecordFailure(ctx context.Context, runID int64, event *run.Event) error
	RecoverStaleRuns(ctx context.Context) (int, error)
	PurgeExpiredEvents(ctx context.Context, retention time.Duration) (int64, error)
}

// ImportResult counts the outcome of one import.
type ImportResult struct {
	RunID     int64
	Imported  int
	Duplicate int
	Failed    int
}

// jsonRecord is the wire form of one JSONL line.
type jsonRecord struct {
	ExternalID   string         `json:"external_id"`
	Title        string         `json:"title"`
	Body         string         `json:"body"`
	MIMEType     string         `json:"mime_type"`
	CanonicalURL string         `json:"canonical_url"`
	OccurredAt   string         `json:"occurred_at"`
	Metadata     map[string]any `json:"metadata"`
}

// importer carries the per-run state of one import.
type importer struct {
	store   RunStore
	logger  *slog.Logger
	runID   int64
	options Options

	events        int
	lastHeartbeat time.Time
}

// ImportJSONL reads JSONL candidates, stores them idempotently, and records the
// run so the outcome stays queryable after the process exits.
//
// A malformed or invalid line is counted as failed and continues. A storage
// failure aborts: the process can no longer trust what it writes next.
func ImportJSONL(
	ctx context.Context,
	store RunStore,
	input io.Reader,
	logger *slog.Logger,
	sourceName string,
	options Options,
) (ImportResult, error) {
	var result ImportResult

	// Housekeeping before the run: reclaim abandoned runs and drop expired
	// events. Both are cheap and keep the database honest without a scheduler.
	if recovered, err := store.RecoverStaleRuns(ctx); err != nil {
		return result, err
	} else if recovered > 0 {
		logger.Warn("marked stale runs as interrupted", "count", recovered)
	}
	if purged, err := store.PurgeExpiredEvents(ctx, options.EventRetention); err != nil {
		return result, err
	} else if purged > 0 {
		logger.Debug("purged expired run events", "count", purged)
	}

	sourceID, err := store.EnsureSource(ctx, sourceName, ManualConnectorKind)
	if err != nil {
		return result, err
	}

	runID, err := store.StartRun(ctx, sourceID, run.TriggerManual)
	if err != nil {
		return result, err
	}
	result.RunID = runID

	imp := &importer{
		store:         store,
		logger:        logger.With("run_id", runID, "source", sourceName),
		runID:         runID,
		options:       options,
		lastHeartbeat: time.Now(),
	}
	imp.logger.Info("import started")
	imp.event(ctx, run.Event{
		Level:   run.LevelInfo,
		Message: "import started",
		Detail:  map[string]any{"source": sourceName},
	})

	counts, err := imp.consume(ctx, input, sourceID)
	result.Imported = counts.Imported
	result.Duplicate = counts.Duplicate
	result.Failed = counts.Failed

	status, failureMessage := outcome(ctx, err, counts)
	if err != nil {
		imp.logger.Error("import aborted", "error", err)
		imp.event(ctx, run.Event{
			Level:   run.LevelError,
			Message: failureMessage,
			Detail:  map[string]any{"error": err.Error()},
		})
	} else {
		imp.logger.Info("import finished",
			"imported", counts.Imported,
			"duplicate", counts.Duplicate,
			"failed", counts.Failed)
		imp.event(ctx, run.Event{
			Level:   run.LevelInfo,
			Message: "import finished",
			Detail: map[string]any{
				"imported":  counts.Imported,
				"duplicate": counts.Duplicate,
				"failed":    counts.Failed,
			},
		})
	}

	// Closing the run uses a detached context so that a cancelled import still
	// leaves a terminal status rather than a run that looks abandoned.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if finishErr := store.FinishRun(finishCtx, runID, status, failureMessage); finishErr != nil {
		if err == nil {
			return result, finishErr
		}
		imp.logger.Error("could not record run outcome", "error", finishErr)
	}

	return result, err
}

// consume reads every line and returns what landed.
func (i *importer) consume(ctx context.Context, input io.Reader, sourceID int64) (run.Counts, error) {
	var counts run.Counts

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLineBytes)

	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		if err := ctx.Err(); err != nil {
			return counts, err
		}
		i.maybeHeartbeat(ctx)

		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		rec, err := decodeLine(line)
		if err == nil {
			rec, err = rec.Normalize(i.options.MaxBodyBytes)
		}
		if err != nil {
			counts.Failed++
			if failErr := i.recordFailure(ctx, lineNumber, err); failErr != nil {
				return counts, failErr
			}
			continue
		}

		inserted, err := i.store.AddCapture(ctx, sourceID, i.runID, rec)
		if err != nil {
			return counts, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if inserted {
			counts.Imported++
		} else {
			counts.Duplicate++
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return counts, fmt.Errorf("a line exceeds the %d byte limit", maxLineBytes)
		}
		return counts, fmt.Errorf("read input: %w", err)
	}
	return counts, nil
}

// recordFailure counts a bad line and stores its reason until the event budget
// is spent, after which only the count keeps rising.
func (i *importer) recordFailure(ctx context.Context, lineNumber int, cause error) error {
	i.logger.Warn("line rejected", "line", lineNumber, "error", cause)

	if i.events >= run.MaxEventsPerRun {
		return i.store.RecordFailure(ctx, i.runID, nil)
	}

	event := &run.Event{
		Level:   run.LevelError,
		Message: fmt.Sprintf("line %d rejected: %v", lineNumber, cause),
		Detail:  map[string]any{"line": lineNumber},
	}
	if err := i.store.RecordFailure(ctx, i.runID, event); err != nil {
		return err
	}
	i.events++
	i.noteTruncation(ctx)
	return nil
}

// event stores one lifecycle event, respecting the same budget.
func (i *importer) event(ctx context.Context, event run.Event) {
	if i.events >= run.MaxEventsPerRun {
		return
	}
	if err := i.store.AppendEvent(ctx, i.runID, event); err != nil {
		i.logger.Error("could not store run event", "error", err)
		return
	}
	i.events++
	i.noteTruncation(ctx)
}

// noteTruncation leaves a marker the moment the budget runs out, so a reader
// can tell a quiet run from a truncated one.
func (i *importer) noteTruncation(ctx context.Context) {
	if i.events != run.MaxEventsPerRun {
		return
	}
	// Written directly: the budget is already spent, and this marker explains why.
	if err := i.store.AppendEvent(ctx, i.runID, run.Event{
		Level:   run.LevelWarn,
		Message: "event log truncated: this run reached its event limit",
		Detail:  map[string]any{"limit": run.MaxEventsPerRun},
	}); err != nil {
		i.logger.Error("could not store truncation notice", "error", err)
		return
	}
	i.events++
}

func (i *importer) maybeHeartbeat(ctx context.Context) {
	if time.Since(i.lastHeartbeat) < run.HeartbeatInterval {
		return
	}
	i.lastHeartbeat = time.Now()
	if err := i.store.Heartbeat(ctx, i.runID); err != nil {
		i.logger.Warn("could not refresh heartbeat", "error", err)
	}
}

// outcome maps how the import ended onto the run state machine.
func outcome(ctx context.Context, err error, counts run.Counts) (run.Status, string) {
	switch {
	case err == nil:
		// Rejected lines are line-level failures, not run-level ones.
		return run.Succeeded, ""
	case errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled):
		return run.Canceled, "import canceled"
	case counts.Imported > 0 || counts.Duplicate > 0:
		// Something already committed, so the run is partially done.
		return run.Partial, "import aborted after committing some records"
	default:
		return run.Failed, "import aborted before committing any record"
	}
}

func decodeLine(line []byte) (record.Record, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	// Numbers stay exact so that metadata round-trips and hashes consistently.
	decoder.UseNumber()
	// Unknown fields are almost always a typo in a field name, and silently
	// dropping them would silently drop data.
	decoder.DisallowUnknownFields()

	var parsed jsonRecord
	if err := decoder.Decode(&parsed); err != nil {
		return record.Record{}, fmt.Errorf("invalid JSON: %w", err)
	}

	return record.Record{
		ExternalID:   parsed.ExternalID,
		Title:        parsed.Title,
		Body:         parsed.Body,
		MIMEType:     parsed.MIMEType,
		CanonicalURL: parsed.CanonicalURL,
		OccurredAt:   parsed.OccurredAt,
		Metadata:     parsed.Metadata,
	}, nil
}
