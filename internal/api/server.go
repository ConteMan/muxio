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
	"sort"
	"strconv"
	"strings"

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

// ConfigWriter persists a configuration, refusing the write when the file no
// longer matches the fingerprint the edit was based on.
type ConfigWriter func(cfg config.Config, expectedFingerprint string) error

// Options carries what the handler needs beyond storage.
type Options struct {
	Version    string
	Store      Reader
	LoadConfig ConfigLoader
	SaveConfig ConfigWriter
	Logger     *slog.Logger
}

type server struct {
	version    string
	store      Reader
	loadConfig ConfigLoader
	saveConfig ConfigWriter
	logger     *slog.Logger
}

// NewHandler creates the local HTTP API handler.
func NewHandler(options Options) http.Handler {
	s := &server{
		version:    options.Version,
		store:      options.Store,
		loadConfig: options.LoadConfig,
		saveConfig: options.SaveConfig,
		logger:     options.Logger,
	}
	if s.logger == nil {
		s.logger = slog.New(slog.DiscardHandler)
	}

	// Each path lists the methods it answers, so a wrong method can be reported
	// with the documented error shape and an accurate Allow header.
	routes := []route{
		{path: "/healthz", handlers: map[string]http.HandlerFunc{"GET": s.health}},
		{path: "/readyz", handlers: map[string]http.HandlerFunc{"GET": s.ready}},
		{path: "/api/v1/status", handlers: map[string]http.HandlerFunc{"GET": s.status}},
		{path: "/api/v1/sources", handlers: map[string]http.HandlerFunc{"GET": s.listSources}},
		{path: "/api/v1/runs", handlers: map[string]http.HandlerFunc{"GET": s.listRuns}},
		{path: "/api/v1/runs/{id}", handlers: map[string]http.HandlerFunc{"GET": s.getRun}},
		{path: "/api/v1/runs/{id}/events", handlers: map[string]http.HandlerFunc{"GET": s.listRunEvents}},
		{path: "/api/v1/config", handlers: map[string]http.HandlerFunc{
			"GET": s.getConfig,
			"PUT": s.putConfig,
		}},
	}

	mux := http.NewServeMux()
	for _, r := range routes {
		methods := make([]string, 0, len(r.handlers))
		for method, handler := range r.handlers {
			mux.HandleFunc(method+" "+r.path, handler)
			methods = append(methods, method)
		}
		sort.Strings(methods)
		// ServeMux would answer a wrong method with a bare body. A method-less
		// pattern keeps the documented shape; the patterns above are more
		// specific and still win.
		mux.HandleFunc(r.path, s.methodNotAllowed(strings.Join(methods, ", ")))
	}
	mux.HandleFunc("/", s.unknown)
	return mux
}

type route struct {
	path     string
	handlers map[string]http.HandlerFunc
}

func (s *server) methodNotAllowed(allowed string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allowed)
		writeError(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed,
			r.Method+" is not supported on this endpoint", "")
	}
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
