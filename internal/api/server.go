// Package api serves the local HTTP contract described by api/openapi.yaml.
//
// Handlers translate between HTTP and the application; they carry no domain
// rules of their own.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ConteMan/muxio/internal/config"
	"github.com/ConteMan/muxio/internal/store/sqlite"
)

// Paging bounds. A request above the maximum is clamped rather than rejected:
// a client asking for too much wants as much as it can get.
const (
	defaultLimit = 20
	maxLimit     = 100
)

// Reader is the read-side storage capability the API needs.
type Reader interface {
	Ping(ctx context.Context) error
	ListSources(ctx context.Context, page sqlite.Page) ([]sqlite.SourceSummary, error)
	ListRunsPage(ctx context.Context, page sqlite.Page, filter sqlite.RunFilter) ([]sqlite.RunSummary, error)
	GetRun(ctx context.Context, runID int64) (sqlite.RunSummary, []sqlite.EventRecord, error)
	ListRunEvents(ctx context.Context, runID int64, page sqlite.Page) ([]sqlite.EventRecord, error)
	RunExists(ctx context.Context, runID int64) (bool, error)
}

// ConfigLoader reports the effective configuration and where it came from.
type ConfigLoader func() (config.Loaded, error)

// Options carries what the handler needs beyond storage.
type Options struct {
	Version    string
	Store      Reader
	LoadConfig ConfigLoader
	Logger     *slog.Logger
}

type server struct {
	version    string
	store      Reader
	loadConfig ConfigLoader
	logger     *slog.Logger
}

// NewHandler creates the local HTTP API handler.
func NewHandler(options Options) http.Handler {
	s := &server{
		version:    options.Version,
		store:      options.Store,
		loadConfig: options.LoadConfig,
		logger:     options.Logger,
	}
	if s.logger == nil {
		s.logger = slog.New(slog.DiscardHandler)
	}

	routes := map[string]http.HandlerFunc{
		"/healthz":                 s.health,
		"/readyz":                  s.ready,
		"/api/v1/status":           s.status,
		"/api/v1/sources":          s.listSources,
		"/api/v1/runs":             s.listRuns,
		"/api/v1/runs/{id}":        s.getRun,
		"/api/v1/runs/{id}/events": s.listRunEvents,
		"/api/v1/config":           s.getConfig,
	}

	mux := http.NewServeMux()
	for path, handler := range routes {
		mux.HandleFunc("GET "+path, handler)
		// ServeMux would answer a wrong method with a bare body. Registering a
		// method-less pattern per path lets the response keep the documented
		// error shape; the GET pattern above is more specific and still wins.
		mux.HandleFunc(path, s.methodNotAllowed)
	}
	mux.HandleFunc("/", s.unknown)
	return mux
}

func (s *server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", http.MethodGet)
	writeError(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed,
		r.Method+" is not supported on this endpoint", "")
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready reports storage reachability, which is what a client needs to know
// before trusting any other endpoint.
func (s *server) ready(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternal,
			"storage is not configured", "")
		return
	}
	if err := s.store.Ping(r.Context()); err != nil {
		s.logger.Error("readiness check failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, CodeInternal,
			"storage is unreachable", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": s.version,
	})
}

func (s *server) unknown(w http.ResponseWriter, r *http.Request) {
	notFound(w, "no such endpoint: "+r.URL.Path)
}

func (s *server) listSources(w http.ResponseWriter, r *http.Request) {
	page, ok := parsePage(w, r)
	if !ok {
		return
	}

	sources, err := s.store.ListSources(r.Context(), page)
	if err != nil {
		internalError(w, s.logFailure(r), err)
		return
	}

	items := make([]sourceView, 0, len(sources))
	for _, source := range sources {
		items = append(items, sourceView{
			ID:            source.ID,
			Name:          source.Name,
			ConnectorKind: source.ConnectorKind,
			Enabled:       source.Enabled,
			CreatedAt:     source.CreatedAt,
			UpdatedAt:     source.UpdatedAt,
		})
	}

	var nextBefore *int64
	if len(items) == page.Limit {
		nextBefore = &items[len(items)-1].ID
	}
	writeJSON(w, http.StatusOK, sourcePage{Items: items, NextBefore: nextBefore})
}

func (s *server) listRuns(w http.ResponseWriter, r *http.Request) {
	page, ok := parsePage(w, r)
	if !ok {
		return
	}

	var filter sqlite.RunFilter
	if raw := r.URL.Query().Get("source_id"); raw != "" {
		sourceID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || sourceID < 1 {
			invalidArgument(w, "source_id", "source_id must be a positive whole number")
			return
		}
		filter.SourceID = sourceID
	}

	runs, err := s.store.ListRunsPage(r.Context(), page, filter)
	if err != nil {
		internalError(w, s.logFailure(r), err)
		return
	}

	items := make([]runView, 0, len(runs))
	for _, summary := range runs {
		items = append(items, newRunView(summary))
	}

	var nextBefore *int64
	if len(items) == page.Limit {
		nextBefore = &items[len(items)-1].ID
	}
	writeJSON(w, http.StatusOK, runPage{Items: items, NextBefore: nextBefore})
}

func (s *server) getRun(w http.ResponseWriter, r *http.Request) {
	runID, ok := parseRunID(w, r)
	if !ok {
		return
	}

	summary, _, err := s.store.GetRun(r.Context(), runID)
	if err != nil {
		if isNotFound(err) {
			notFound(w, "no run with id "+strconv.FormatInt(runID, 10))
			return
		}
		internalError(w, s.logFailure(r), err)
		return
	}
	writeJSON(w, http.StatusOK, newRunView(summary))
}

func (s *server) listRunEvents(w http.ResponseWriter, r *http.Request) {
	runID, ok := parseRunID(w, r)
	if !ok {
		return
	}
	page, ok := parsePage(w, r)
	if !ok {
		return
	}

	// A run with no events and an unknown run are different answers.
	exists, err := s.store.RunExists(r.Context(), runID)
	if err != nil {
		internalError(w, s.logFailure(r), err)
		return
	}
	if !exists {
		notFound(w, "no run with id "+strconv.FormatInt(runID, 10))
		return
	}

	events, err := s.store.ListRunEvents(r.Context(), runID, page)
	if err != nil {
		internalError(w, s.logFailure(r), err)
		return
	}

	items := make([]runEventView, 0, len(events))
	for _, event := range events {
		items = append(items, newRunEventView(event))
	}

	var nextBefore *int64
	if len(items) == page.Limit {
		nextBefore = &items[len(items)-1].ID
	}
	writeJSON(w, http.StatusOK, runEventPage{Items: items, NextBefore: nextBefore})
}

func (s *server) getConfig(w http.ResponseWriter, r *http.Request) {
	if s.loadConfig == nil {
		internalError(w, s.logFailure(r), errNoConfigLoader)
		return
	}

	loaded, err := s.loadConfig()
	if err != nil {
		internalError(w, s.logFailure(r), err)
		return
	}

	settings := make([]configSettingView, 0, len(config.Keys()))
	for _, key := range config.Keys() {
		value, err := config.Get(loaded.Config, key)
		if err != nil {
			internalError(w, s.logFailure(r), err)
			return
		}
		settings = append(settings, configSettingView{
			Key:    key,
			Value:  value,
			Origin: string(loaded.Origin(key)),
		})
	}

	writeJSON(w, http.StatusOK, configView{
		Path:     loaded.Path,
		Exists:   loaded.Exists,
		Settings: settings,
	})
}

func (s *server) logFailure(r *http.Request) func(error) {
	return func(err error) {
		s.logger.Error("request failed",
			"method", r.Method, "path", r.URL.Path, "error", err)
	}
}

// parsePage reads the shared paging parameters.
func parsePage(w http.ResponseWriter, r *http.Request) (sqlite.Page, bool) {
	query := r.URL.Query()
	page := sqlite.Page{Limit: defaultLimit}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			invalidArgument(w, "limit", "limit must be a whole number")
			return sqlite.Page{}, false
		}
		if limit < 1 {
			invalidArgument(w, "limit", "limit must be at least 1")
			return sqlite.Page{}, false
		}
		page.Limit = min(limit, maxLimit)
	}

	if raw := query.Get("before"); raw != "" {
		before, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			invalidArgument(w, "before", "before must be a positive whole number")
			return sqlite.Page{}, false
		}
		page.Before = before
	}

	return page, true
}

func parseRunID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	runID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || runID < 1 {
		invalidArgument(w, "id", "run id must be a positive whole number")
		return 0, false
	}
	return runID, true
}

// detailObject decodes stored event detail so clients receive an object rather
// than a string holding JSON.
func detailObject(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		// Detail is explanatory, not load-bearing: surface it rather than fail.
		return map[string]any{"raw": raw}
	}
	return decoded
}
