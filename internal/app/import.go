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

	"github.com/ConteMan/muxio/internal/record"
)

// ManualConnectorKind marks sources created by hand rather than by a connector.
const ManualConnectorKind = "manual"

// maxLineBytes bounds one JSONL line. A 5 MiB body can expand under JSON
// escaping, so the line budget is larger than the body limit.
const maxLineBytes = 32 << 20

// CaptureWriter is the storage capability an import needs.
type CaptureWriter interface {
	EnsureSource(ctx context.Context, name, connectorKind string) (int64, error)
	AddCapture(ctx context.Context, sourceID int64, rec record.Record) (bool, error)
}

// ImportResult counts the outcome of one import.
type ImportResult struct {
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

// ImportJSONL reads JSONL candidates and stores them idempotently.
//
// A malformed or invalid line is counted as failed and reported to problems,
// then the import continues. A storage failure aborts: it means the process
// cannot trust anything it writes next.
func ImportJSONL(
	ctx context.Context,
	store CaptureWriter,
	input io.Reader,
	problems io.Writer,
	sourceName string,
) (ImportResult, error) {
	var result ImportResult

	sourceID, err := store.EnsureSource(ctx, sourceName, ManualConnectorKind)
	if err != nil {
		return result, err
	}

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLineBytes)

	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		rec, err := decodeLine(line)
		if err != nil {
			result.Failed++
			reportLine(problems, lineNumber, err)
			continue
		}

		normalized, err := rec.Normalize()
		if err != nil {
			result.Failed++
			reportLine(problems, lineNumber, err)
			continue
		}

		inserted, err := store.AddCapture(ctx, sourceID, normalized)
		if err != nil {
			return result, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if inserted {
			result.Imported++
		} else {
			result.Duplicate++
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return result, fmt.Errorf("a line exceeds the %d byte limit", maxLineBytes)
		}
		return result, fmt.Errorf("read input: %w", err)
	}
	return result, nil
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

func reportLine(problems io.Writer, lineNumber int, err error) {
	if problems == nil {
		return
	}
	_, _ = fmt.Fprintf(problems, "line %d: %v\n", lineNumber, err)
}
