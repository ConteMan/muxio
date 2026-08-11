package api

import (
	"errors"

	"github.com/ConteMan/muxio/internal/store/sqlite"
)

// errNoConfigLoader reports a handler built without configuration access.
var errNoConfigLoader = errors.New("no configuration loader was provided")

// errNoConfigWriter reports a handler built without the ability to persist.
var errNoConfigWriter = errors.New("no configuration writer was provided")

// The view types below are the published wire shapes. They are deliberately
// separate from the storage structs so a column rename cannot silently change
// the contract in api/openapi.yaml.

type sourceView struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	ConnectorKind string `json:"connector_kind"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type sourcePage struct {
	Items      []sourceView `json:"items"`
	NextBefore *int64       `json:"next_before"`
}

type runView struct {
	ID             int64   `json:"id"`
	SourceID       int64   `json:"source_id"`
	SourceName     string  `json:"source_name"`
	Trigger        string  `json:"trigger"`
	Status         string  `json:"status"`
	StartedAt      string  `json:"started_at"`
	FinishedAt     *string `json:"finished_at"`
	ImportedCount  int     `json:"imported_count"`
	DuplicateCount int     `json:"duplicate_count"`
	FailedCount    int     `json:"failed_count"`
	Attempt        int     `json:"attempt"`
	LastError      *string `json:"last_error"`
}

type runPage struct {
	Items      []runView `json:"items"`
	NextBefore *int64    `json:"next_before"`
}

type runEventView struct {
	ID         int64          `json:"id"`
	RunID      int64          `json:"run_id"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	Detail     map[string]any `json:"detail"`
	OccurredAt string         `json:"occurred_at"`
}

type runEventPage struct {
	Items      []runEventView `json:"items"`
	NextBefore *int64         `json:"next_before"`
}

type configSettingView struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Origin string `json:"origin"`
}

type configView struct {
	Path     string              `json:"path"`
	Exists   bool                `json:"exists"`
	Settings []configSettingView `json:"settings"`
}

func newRunView(summary sqlite.RunSummary) runView {
	return runView{
		ID:             summary.ID,
		SourceID:       summary.SourceID,
		SourceName:     summary.SourceName,
		Trigger:        summary.Trigger,
		Status:         string(summary.Status),
		StartedAt:      summary.StartedAt,
		FinishedAt:     optional(summary.FinishedAt),
		ImportedCount:  summary.Counts.Imported,
		DuplicateCount: summary.Counts.Duplicate,
		FailedCount:    summary.Counts.Failed,
		Attempt:        summary.Attempt,
		LastError:      optional(summary.LastError),
	}
}

func newRunEventView(event sqlite.EventRecord) runEventView {
	return runEventView{
		ID:         event.ID,
		RunID:      event.RunID,
		Level:      event.Level,
		Message:    event.Message,
		Detail:     detailObject(event.Detail),
		OccurredAt: event.OccurredAt,
	}
}

// optional turns storage's empty string into an explicit JSON null, so a client
// can tell "not finished" from "finished at the zero time".
func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func isNotFound(err error) bool {
	return errors.Is(err, sqlite.ErrRunNotFound)
}
