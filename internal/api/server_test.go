package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ConteMan/muxio/internal/config"
	"github.com/ConteMan/muxio/internal/record"
	"github.com/ConteMan/muxio/internal/run"
	"github.com/ConteMan/muxio/internal/store/sqlite"
)

func TestHealthIgnoresStorage(t *testing.T) {
	// Liveness must not depend on the database, or a storage outage would look
	// like a dead process to a supervisor.
	recorder := get(t, NewHandler(Options{Version: "dev"}), "/healthz")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
}

func TestReadyReflectsStorage(t *testing.T) {
	handler, _ := newTestHandler(t)
	if recorder := get(t, handler, "/readyz"); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	// Without storage the service is not ready, and says so distinctly.
	recorder := get(t, NewHandler(Options{Version: "dev"}), "/readyz")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	assertErrorShape(t, recorder, CodeInternal)
}

func TestStatusIncludesVersion(t *testing.T) {
	recorder := get(t, NewHandler(Options{Version: "v0.1.0-test"}), "/api/v1/status")

	var response struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	decode(t, recorder, &response)
	if response.Version != "v0.1.0-test" || response.Status != "ok" {
		t.Fatalf("response = %+v", response)
	}
}

func TestListSources(t *testing.T) {
	handler, store := newTestHandler(t)
	seedSource(t, store, "notes")
	seedSource(t, store, "feeds")

	var page struct {
		Items      []map[string]any `json:"items"`
		NextBefore *int64           `json:"next_before"`
	}
	decode(t, get(t, handler, "/api/v1/sources"), &page)

	if len(page.Items) != 2 {
		t.Fatalf("listed %d sources, want 2", len(page.Items))
	}
	// Newest first.
	if page.Items[0]["name"] != "feeds" {
		t.Fatalf("first item = %v, want the newest source", page.Items[0]["name"])
	}
	if page.Items[0]["connector_kind"] != "manual" {
		t.Errorf("connector_kind = %v", page.Items[0]["connector_kind"])
	}
	if page.NextBefore != nil {
		t.Errorf("next_before = %v, want null on the last page", *page.NextBefore)
	}
}

func TestListRunsMatchesStoredRun(t *testing.T) {
	handler, store := newTestHandler(t)
	ctx := context.Background()
	sourceID := seedSource(t, store, "notes")

	runID, err := store.StartRun(ctx, sourceID, run.TriggerManual)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	addCapture(t, store, sourceID, runID, "note-1")
	if err := store.FinishRun(ctx, runID, run.Succeeded, ""); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	var page struct {
		Items []map[string]any `json:"items"`
	}
	decode(t, get(t, handler, "/api/v1/runs"), &page)

	if len(page.Items) != 1 {
		t.Fatalf("listed %d runs, want 1", len(page.Items))
	}
	item := page.Items[0]
	if item["status"] != string(run.Succeeded) {
		t.Errorf("status = %v", item["status"])
	}
	if item["source_name"] != "notes" {
		t.Errorf("source_name = %v", item["source_name"])
	}
	if item["imported_count"] != float64(1) {
		t.Errorf("imported_count = %v", item["imported_count"])
	}
	if item["finished_at"] == nil {
		t.Error("finished_at is null for a finished run")
	}
	// An unfinished field must be an explicit null, not an empty string.
	if item["last_error"] != nil {
		t.Errorf("last_error = %v, want null", item["last_error"])
	}
}

// A running run has no finish time, and the wire form must say so as null.
func TestRunningRunHasNullFinishedAt(t *testing.T) {
	handler, store := newTestHandler(t)
	sourceID := seedSource(t, store, "notes")
	runID := startRun(t, store, sourceID)

	var view map[string]any
	decode(t, get(t, handler, "/api/v1/runs/"+itoa(runID)), &view)

	if view["finished_at"] != nil {
		t.Fatalf("finished_at = %v, want null", view["finished_at"])
	}
	if view["status"] != string(run.Running) {
		t.Fatalf("status = %v", view["status"])
	}
}

func TestGetRunNotFound(t *testing.T) {
	handler, _ := newTestHandler(t)

	recorder := get(t, handler, "/api/v1/runs/999")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	assertErrorShape(t, recorder, CodeNotFound)
}

// Paging must walk the whole history without repeating or dropping a run.
func TestRunPagingIsCompleteAndStable(t *testing.T) {
	handler, store := newTestHandler(t)
	sourceID := seedSource(t, store, "notes")

	const total = 5
	for range total {
		startRun(t, store, sourceID)
	}

	seen := map[float64]bool{}
	path := "/api/v1/runs?limit=1"
	for range total + 2 {
		var page struct {
			Items      []map[string]any `json:"items"`
			NextBefore *int64           `json:"next_before"`
		}
		decode(t, get(t, handler, path), &page)

		for _, item := range page.Items {
			id := item["id"].(float64)
			if seen[id] {
				t.Fatalf("run %v appeared twice", id)
			}
			seen[id] = true
		}
		if page.NextBefore == nil {
			break
		}
		path = "/api/v1/runs?limit=1&before=" + itoa(*page.NextBefore)
	}

	if len(seen) != total {
		t.Fatalf("walked %d runs, want %d", len(seen), total)
	}
}

func TestRunsFilterBySource(t *testing.T) {
	handler, store := newTestHandler(t)
	notes := seedSource(t, store, "notes")
	feeds := seedSource(t, store, "feeds")
	startRun(t, store, notes)
	startRun(t, store, feeds)
	startRun(t, store, feeds)

	var page struct {
		Items []map[string]any `json:"items"`
	}
	decode(t, get(t, handler, "/api/v1/runs?source_id="+itoa(feeds)), &page)

	if len(page.Items) != 2 {
		t.Fatalf("listed %d runs, want the 2 from one source", len(page.Items))
	}
	for _, item := range page.Items {
		if item["source_name"] != "feeds" {
			t.Fatalf("run from %v leaked into the filtered page", item["source_name"])
		}
	}
}

func TestListRunEvents(t *testing.T) {
	handler, store := newTestHandler(t)
	ctx := context.Background()
	sourceID := seedSource(t, store, "notes")
	runID := startRun(t, store, sourceID)

	if err := store.AppendEvent(ctx, runID, run.Event{
		Level:   run.LevelError,
		Message: "line 3 rejected",
		Detail:  map[string]any{"line": 3},
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	var page struct {
		Items []map[string]any `json:"items"`
	}
	decode(t, get(t, handler, "/api/v1/runs/"+itoa(runID)+"/events"), &page)

	if len(page.Items) != 1 {
		t.Fatalf("listed %d events, want 1", len(page.Items))
	}
	event := page.Items[0]
	if event["message"] != "line 3 rejected" {
		t.Errorf("message = %v", event["message"])
	}
	// Detail must arrive as an object, not as a string holding JSON.
	detail, ok := event["detail"].(map[string]any)
	if !ok {
		t.Fatalf("detail = %T, want an object", event["detail"])
	}
	if detail["line"] != float64(3) {
		t.Errorf("detail.line = %v", detail["line"])
	}
}

// An unknown run and a run with no events are different answers.
func TestEventsDistinguishEmptyFromMissing(t *testing.T) {
	handler, store := newTestHandler(t)
	sourceID := seedSource(t, store, "notes")
	runID := startRun(t, store, sourceID)

	var page struct {
		Items []map[string]any `json:"items"`
	}
	recorder := get(t, handler, "/api/v1/runs/"+itoa(runID)+"/events")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d for a run without events", recorder.Code, http.StatusOK)
	}
	decode(t, recorder, &page)
	if len(page.Items) != 0 {
		t.Fatalf("listed %d events, want an empty page", len(page.Items))
	}

	missing := get(t, handler, "/api/v1/runs/4242/events")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d for an unknown run", missing.Code, http.StatusNotFound)
	}
}

func TestInvalidParametersAreRejected(t *testing.T) {
	handler, _ := newTestHandler(t)

	tests := map[string]string{
		"/api/v1/runs?limit=0":       "limit",
		"/api/v1/runs?limit=abc":     "limit",
		"/api/v1/runs?before=abc":    "before",
		"/api/v1/runs?before=0":      "before",
		"/api/v1/runs?source_id=abc": "source_id",
		"/api/v1/runs/abc":           "id",
		"/api/v1/runs/0":             "id",
	}

	for path, wantField := range tests {
		t.Run(path, func(t *testing.T) {
			recorder := get(t, handler, path)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			body := assertErrorShape(t, recorder, CodeInvalidArgument)
			if body.Field != wantField {
				t.Fatalf("field = %q, want %q", body.Field, wantField)
			}
		})
	}
}

// Asking for more than the maximum yields the maximum, not an error.
func TestLimitIsClamped(t *testing.T) {
	handler, store := newTestHandler(t)
	sourceID := seedSource(t, store, "notes")
	for range 3 {
		startRun(t, store, sourceID)
	}

	recorder := get(t, handler, "/api/v1/runs?limit=9999")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var page struct {
		Items      []map[string]any `json:"items"`
		NextBefore *int64           `json:"next_before"`
	}
	decode(t, recorder, &page)
	if len(page.Items) != 3 {
		t.Fatalf("listed %d runs, want all 3", len(page.Items))
	}
	if page.NextBefore != nil {
		t.Fatalf("next_before = %v, want null when the page is not full", *page.NextBefore)
	}
}

func TestConfigReportsValuesAndOrigins(t *testing.T) {
	store := newTestStore(t)
	handler := NewHandler(Options{
		Version: "dev",
		Store:   store,
		LoadConfig: func() (config.Loaded, error) {
			loaded := config.Loaded{
				Config:  config.Default(),
				Origins: map[string]config.Origin{"log.level": config.FromEnv},
				Path:    "/tmp/config.toml",
				Exists:  true,
			}
			loaded.Config.Log.Level = "debug"
			return loaded, nil
		},
	})

	var view struct {
		Path     string `json:"path"`
		Exists   bool   `json:"exists"`
		Settings []struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Origin string `json:"origin"`
		} `json:"settings"`
	}
	decode(t, get(t, handler, "/api/v1/config"), &view)

	if view.Path != "/tmp/config.toml" || !view.Exists {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Settings) != len(config.Keys()) {
		t.Fatalf("returned %d settings, want %d", len(view.Settings), len(config.Keys()))
	}

	origins := map[string]string{}
	values := map[string]string{}
	for _, setting := range view.Settings {
		origins[setting.Key] = setting.Origin
		values[setting.Key] = setting.Value
	}
	if origins["log.level"] != string(config.FromEnv) {
		t.Errorf("log.level origin = %q, want env", origins["log.level"])
	}
	if values["log.level"] != "debug" {
		t.Errorf("log.level value = %q", values["log.level"])
	}
	// Fields not overridden report the default origin.
	if origins["server.addr"] != string(config.FromDefault) {
		t.Errorf("server.addr origin = %q, want default", origins["server.addr"])
	}
}

func TestUnknownPathAndMethod(t *testing.T) {
	handler, _ := newTestHandler(t)

	recorder := get(t, handler, "/api/v1/nope")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	assertErrorShape(t, recorder, CodeNotFound)

	post := httptest.NewRequest(http.MethodPost, "/api/v1/runs", nil)
	posted := httptest.NewRecorder()
	handler.ServeHTTP(posted, post)
	if posted.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", posted.Code, http.StatusMethodNotAllowed)
	}
}

// ---- helpers ----

func newTestHandler(t *testing.T) (http.Handler, *sqlite.Store) {
	t.Helper()
	store := newTestStore(t)
	handler := NewHandler(Options{
		Version:    "dev",
		Store:      store,
		LoadConfig: func() (config.Loaded, error) { return config.Load("/nonexistent", emptyEnv) },
	})
	return handler, store
}

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	ctx := context.Background()

	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "muxio.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func seedSource(t *testing.T, store *sqlite.Store, name string) int64 {
	t.Helper()
	sourceID, err := store.EnsureSource(context.Background(), name, "manual")
	if err != nil {
		t.Fatalf("EnsureSource: %v", err)
	}
	return sourceID
}

func startRun(t *testing.T, store *sqlite.Store, sourceID int64) int64 {
	t.Helper()
	runID, err := store.StartRun(context.Background(), sourceID, run.TriggerManual)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	return runID
}

func addCapture(t *testing.T, store *sqlite.Store, sourceID, runID int64, externalID string) {
	t.Helper()
	rec, err := record.Record{ExternalID: externalID, Body: "body"}.
		Normalize(record.DefaultMaxBodyBytes)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if _, err := store.AddCapture(context.Background(), sourceID, runID, rec); err != nil {
		t.Fatalf("AddCapture: %v", err)
	}
}

func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder, into any) {
	t.Helper()
	if err := json.NewDecoder(recorder.Body).Decode(into); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertErrorShape(t *testing.T, recorder *httptest.ResponseRecorder, wantCode string) errorBody {
	t.Helper()
	var body errorBody
	decode(t, recorder, &body)
	if body.Error != wantCode {
		t.Fatalf("error = %q, want %q", body.Error, wantCode)
	}
	if body.Message == "" {
		t.Fatal("error response has no message")
	}
	return body
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}

func emptyEnv(string) (string, bool) { return "", false }
