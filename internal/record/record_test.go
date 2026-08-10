package record

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeAppliesCanonicalForm(t *testing.T) {
	normalized, err := Record{
		ExternalID: "  note-1  ",
		Title:      "  hello\r\nworld  ",
		Body:       "line one\r\nline two\r",
		OccurredAt: "2026-08-10T18:00:00+08:00",
	}.Normalize()
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	if normalized.ExternalID != "note-1" {
		t.Errorf("ExternalID = %q", normalized.ExternalID)
	}
	if normalized.Title != "hello\nworld" {
		t.Errorf("Title = %q", normalized.Title)
	}
	if normalized.Body != "line one\nline two" {
		t.Errorf("Body = %q", normalized.Body)
	}
	if normalized.MIMEType != "text/plain" {
		t.Errorf("MIMEType = %q", normalized.MIMEType)
	}
	if normalized.OccurredAt != "2026-08-10T10:00:00Z" {
		t.Errorf("OccurredAt = %q, want the UTC form", normalized.OccurredAt)
	}
}

func TestNormalizeRejectsInvalidRecords(t *testing.T) {
	tests := []struct {
		name    string
		record  Record
		wantErr error
	}{
		{"missing external id", Record{ExternalID: "   "}, ErrMissingExternalID},
		{"oversized body", Record{
			ExternalID: "big",
			Body:       strings.Repeat("x", MaxBodyBytes+1),
		}, ErrBodyTooLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.record.Normalize(); !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestNormalizeKeepsUnknownTimeEmpty(t *testing.T) {
	normalized, err := Record{ExternalID: "note"}.Normalize()
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if normalized.OccurredAt != "" {
		t.Fatalf("OccurredAt = %q, want empty rather than a guess", normalized.OccurredAt)
	}

	if _, err := (Record{ExternalID: "note", OccurredAt: "10/08/2026"}).Normalize(); err == nil {
		t.Fatal("a non-RFC3339 timestamp was accepted")
	}
}

func TestContentHashIsStableAcrossEquivalentInput(t *testing.T) {
	first := mustNormalize(t, Record{
		ExternalID: "note-1",
		Title:      "Title",
		Body:       "body\r\ntext",
		Metadata:   map[string]any{"b": "2", "a": "1"},
	})
	// Same content, different line endings, different metadata ordering.
	second := mustNormalize(t, Record{
		ExternalID: "note-1",
		Title:      "Title",
		Body:       "body\ntext",
		Metadata:   map[string]any{"a": "1", "b": "2"},
	})

	if hashOf(t, first) != hashOf(t, second) {
		t.Fatal("equivalent records produced different hashes")
	}
}

func TestContentHashChangesWithContent(t *testing.T) {
	base := mustNormalize(t, Record{ExternalID: "note-1", Title: "Title", Body: "body"})

	variants := map[string]Record{
		"body":          {ExternalID: "note-1", Title: "Title", Body: "body changed"},
		"title":         {ExternalID: "note-1", Title: "Changed", Body: "body"},
		"canonical_url": {ExternalID: "note-1", Title: "Title", Body: "body", CanonicalURL: "https://example.test"},
		"occurred_at":   {ExternalID: "note-1", Title: "Title", Body: "body", OccurredAt: "2026-08-10T00:00:00Z"},
		"metadata":      {ExternalID: "note-1", Title: "Title", Body: "body", Metadata: map[string]any{"a": "1"}},
	}

	for field, variant := range variants {
		t.Run(field, func(t *testing.T) {
			if hashOf(t, base) == hashOf(t, mustNormalize(t, variant)) {
				t.Fatalf("changing %s did not change the content hash", field)
			}
		})
	}
}

// The hash is length-prefixed per field so that no shifting of characters
// between adjacent fields can collide.
func TestContentHashResistsFieldBoundaryCollisions(t *testing.T) {
	first := mustNormalize(t, Record{ExternalID: "x", Title: "ab", Body: "c"})
	second := mustNormalize(t, Record{ExternalID: "x", Title: "a", Body: "bc"})

	if hashOf(t, first) == hashOf(t, second) {
		t.Fatal("field boundaries are not encoded in the hash")
	}
}

func TestMetadataJSONIsDeterministic(t *testing.T) {
	rec := Record{Metadata: map[string]any{
		"z": "last",
		"a": "first",
		"n": json.Number("12345678901234567890"),
	}}

	encoded, err := rec.MetadataJSON()
	if err != nil {
		t.Fatalf("MetadataJSON: %v", err)
	}
	want := `{"a":"first","n":12345678901234567890,"z":"last"}`
	if encoded != want {
		t.Fatalf("MetadataJSON() = %s, want %s", encoded, want)
	}

	empty, err := Record{}.MetadataJSON()
	if err != nil {
		t.Fatalf("MetadataJSON: %v", err)
	}
	if empty != "{}" {
		t.Fatalf("empty metadata = %s", empty)
	}
}

func mustNormalize(t *testing.T, rec Record) Record {
	t.Helper()
	normalized, err := rec.Normalize()
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	return normalized
}

func hashOf(t *testing.T, rec Record) string {
	t.Helper()
	contentHash, err := rec.ContentHash()
	if err != nil {
		t.Fatalf("ContentHash: %v", err)
	}
	return contentHash
}
