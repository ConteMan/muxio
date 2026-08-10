// Package record defines a capture candidate and the normalization rules that
// decide when two observations are the same version of the same thing.
package record

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"
)

// MaxBodyBytes bounds a single capture body. Attachments and large binaries are
// out of scope for the first phase, so an oversized body is a rejected record
// rather than a reason to grow the database.
const MaxBodyBytes = 5 << 20

// TimeFormat is the single representation for persisted and API timestamps.
// Display layers are responsible for converting to a local zone.
const TimeFormat = time.RFC3339Nano

var (
	// ErrMissingExternalID reports a record with no stable identity in its source.
	ErrMissingExternalID = errors.New("external_id is required")
	// ErrBodyTooLarge reports a body above MaxBodyBytes.
	ErrBodyTooLarge = errors.New("body exceeds the maximum size")
)

// Record is one capture candidate before it is persisted.
type Record struct {
	ExternalID   string
	Title        string
	Body         string
	MIMEType     string
	CanonicalURL string
	OccurredAt   string
	Metadata     map[string]any
}

// Normalize applies the canonical form and validates the result. Normalization
// feeds the content hash, so changing these rules is equivalent to a migration.
func (r Record) Normalize() (Record, error) {
	normalized := Record{
		ExternalID:   strings.TrimSpace(r.ExternalID),
		Title:        strings.TrimSpace(normalizeNewlines(r.Title)),
		Body:         strings.TrimSpace(normalizeNewlines(r.Body)),
		MIMEType:     strings.TrimSpace(r.MIMEType),
		CanonicalURL: strings.TrimSpace(r.CanonicalURL),
		Metadata:     r.Metadata,
	}

	if normalized.ExternalID == "" {
		return Record{}, ErrMissingExternalID
	}
	if len(normalized.Body) > MaxBodyBytes {
		return Record{}, fmt.Errorf("%w: %d bytes, limit %d",
			ErrBodyTooLarge, len(normalized.Body), MaxBodyBytes)
	}
	if normalized.MIMEType == "" {
		normalized.MIMEType = "text/plain"
	}

	occurredAt, err := normalizeTime(r.OccurredAt)
	if err != nil {
		return Record{}, fmt.Errorf("occurred_at: %w", err)
	}
	normalized.OccurredAt = occurredAt

	return normalized, nil
}

// MetadataJSON renders metadata deterministically. encoding/json sorts map keys,
// so the same metadata always produces the same bytes and the same hash.
func (r Record) MetadataJSON() (string, error) {
	if len(r.Metadata) == 0 {
		return "{}", nil
	}
	encoded, err := json.Marshal(r.Metadata)
	if err != nil {
		return "", fmt.Errorf("encode metadata: %w", err)
	}
	return string(encoded), nil
}

// ContentHash identifies one version of the content. It covers every field that
// carries meaning, and deliberately excludes ExternalID — which is a separate
// part of the deduplication key — and the capture time, which always differs.
func (r Record) ContentHash() (string, error) {
	metadata, err := r.MetadataJSON()
	if err != nil {
		return "", err
	}

	digest := sha256.New()
	writeField(digest, "title", r.Title)
	writeField(digest, "body", r.Body)
	writeField(digest, "mime_type", r.MIMEType)
	writeField(digest, "canonical_url", r.CanonicalURL)
	writeField(digest, "occurred_at", r.OccurredAt)
	writeField(digest, "metadata", metadata)
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// writeField length-prefixes each field so that no combination of values can
// produce the same byte stream as a different combination.
func writeField(digest hash.Hash, name, value string) {
	_, _ = fmt.Fprintf(digest, "%s\x00%d\x00%s\x00", name, len(value), value)
}

func normalizeNewlines(value string) string {
	if !strings.ContainsRune(value, '\r') {
		return value
	}
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
}

// normalizeTime converts an RFC3339 timestamp to UTC. An empty value stays
// empty: an unknown time is not guessed.
func normalizeTime(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return "", fmt.Errorf("expected RFC3339, got %q", trimmed)
	}
	return parsed.UTC().Format(TimeFormat), nil
}

// Now returns the current time in the canonical persisted form.
func Now() string {
	return time.Now().UTC().Format(TimeFormat)
}
