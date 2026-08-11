package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ConteMan/muxio/internal/config"
)

// configFixture wires the handler to a real file so writes are actually
// exercised end to end, including atomicity and permissions.
type configFixture struct {
	handler http.Handler
	path    string
	env     map[string]string
}

func newConfigFixture(t *testing.T) *configFixture {
	t.Helper()

	fixture := &configFixture{
		path: filepath.Join(t.TempDir(), "config.toml"),
		env:  map[string]string{},
	}
	lookup := func(name string) (string, bool) {
		value, ok := fixture.env[name]
		return value, ok
	}
	fixture.handler = NewHandler(Options{
		Version: "dev",
		Store:   newTestStore(t),
		LoadConfig: func() (config.Loaded, error) {
			return config.Load(fixture.path, lookup)
		},
		SaveConfig: func(cfg config.Config, expected string) error {
			return config.Write(fixture.path, cfg, expected)
		},
	})
	return fixture
}

func (f *configFixture) seed(t *testing.T, cfg config.Config) {
	t.Helper()
	if err := os.WriteFile(f.path, []byte(config.Render(cfg)), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

func (f *configFixture) etag(t *testing.T) string {
	t.Helper()
	recorder := get(t, f.handler, "/api/v1/config")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET config status = %d", recorder.Code)
	}
	tag := recorder.Header().Get("ETag")
	if tag == "" {
		t.Fatal("GET config returned no ETag")
	}
	return tag
}

func (f *configFixture) put(t *testing.T, ifMatch string, settings map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	items := make([]configSettingInput, 0, len(settings))
	for key, value := range settings {
		items = append(items, configSettingInput{Key: key, Value: value})
	}
	body, err := json.Marshal(configUpdate{Settings: items})
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	return f.putRaw(t, ifMatch, string(body))
}

func (f *configFixture) putRaw(t *testing.T, ifMatch, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPut, "/api/v1/config", strings.NewReader(body))
	if ifMatch != "" {
		request.Header.Set("If-Match", ifMatch)
	}
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)
	return recorder
}

// fullSettings returns every known setting, which a write must include.
func fullSettings(overrides map[string]string) map[string]string {
	settings := map[string]string{}
	defaults := config.Default()
	for _, key := range config.Keys() {
		value, _ := config.Get(defaults, key)
		settings[key] = value
	}
	for key, value := range overrides {
		settings[key] = value
	}
	return settings
}

func TestConfigETagTracksContent(t *testing.T) {
	fixture := newConfigFixture(t)

	absent := fixture.etag(t)
	fixture.seed(t, config.Default())
	present := fixture.etag(t)

	if absent == present {
		t.Fatal("creating the file did not change the ETag")
	}
	// Repeating an unchanged read yields the same validator.
	if again := fixture.etag(t); again != present {
		t.Fatalf("ETag changed without an edit: %q then %q", present, again)
	}
}

func TestConfigWriteSucceedsWithCurrentETag(t *testing.T) {
	fixture := newConfigFixture(t)
	fixture.seed(t, config.Default())

	recorder := fixture.put(t, fixture.etag(t), fullSettings(map[string]string{
		"log.level": "debug",
	}))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	if recorder.Header().Get("ETag") == "" {
		t.Error("a successful write returned no new ETag")
	}

	content, err := os.ReadFile(fixture.path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), `level = "debug"`) {
		t.Errorf("value was not written:\n%s", content)
	}
	// Generated comments are re-rendered on every write.
	if !strings.Contains(string(content), "# Muxio configuration") {
		t.Errorf("comments were lost:\n%s", content)
	}

	info, err := os.Stat(fixture.path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %04o, want 0600", perm)
	}
}

// An unconditional write is exactly what this endpoint exists to prevent.
func TestConfigWriteRequiresPrecondition(t *testing.T) {
	fixture := newConfigFixture(t)
	fixture.seed(t, config.Default())
	before := fixture.etag(t)

	recorder := fixture.put(t, "", fullSettings(nil))
	if recorder.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusPreconditionRequired)
	}
	assertErrorShape(t, recorder, CodePreconditionRequired)
	if after := fixture.etag(t); after != before {
		t.Fatal("the file was modified despite a missing precondition")
	}
}

// "*" only asserts existence, which would silently discard a concurrent edit.
func TestConfigWriteRejectsWildcardPrecondition(t *testing.T) {
	fixture := newConfigFixture(t)
	fixture.seed(t, config.Default())
	before := fixture.etag(t)

	recorder := fixture.put(t, "*", fullSettings(nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	assertErrorShape(t, recorder, CodeInvalidArgument)
	if after := fixture.etag(t); after != before {
		t.Fatal("the file was modified despite a rejected precondition")
	}
}

// The headline case: a settings page must not clobber a concurrent hand edit.
func TestConfigWriteDetectsConcurrentEdit(t *testing.T) {
	fixture := newConfigFixture(t)
	fixture.seed(t, config.Default())
	stale := fixture.etag(t)

	// Someone edits the file after the page loaded it.
	other := config.Default()
	other.Retention.RunEventDays = 3
	fixture.seed(t, other)

	recorder := fixture.put(t, stale, fullSettings(map[string]string{"log.level": "debug"}))
	if recorder.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusPreconditionFailed)
	}
	assertErrorShape(t, recorder, CodePreconditionFailed)
	// The client is told what to rebase onto.
	if current := recorder.Header().Get("ETag"); current == "" || current == stale {
		t.Fatalf("conflict ETag = %q, want the current one", current)
	}

	// The other edit survived untouched.
	content, err := os.ReadFile(fixture.path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), "run_event_days = 3") {
		t.Errorf("the concurrent edit was overwritten:\n%s", content)
	}
}

// The sentinel means "I believe there is no file", so only one creation wins.
func TestConfigCreationRaceHasOneWinner(t *testing.T) {
	fixture := newConfigFixture(t)
	absent := fixture.etag(t)

	first := fixture.put(t, absent, fullSettings(map[string]string{"log.level": "debug"}))
	if first.Code != http.StatusOK {
		t.Fatalf("first create status = %d, body = %s", first.Code, first.Body)
	}

	second := fixture.put(t, absent, fullSettings(map[string]string{"log.level": "warn"}))
	if second.Code != http.StatusPreconditionFailed {
		t.Fatalf("second create status = %d, want %d", second.Code, http.StatusPreconditionFailed)
	}

	content, err := os.ReadFile(fixture.path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), `level = "debug"`) {
		t.Errorf("the losing write took effect:\n%s", content)
	}
}

// Omitting a setting must fail rather than quietly reset it to its default.
func TestConfigWriteRequiresEverySetting(t *testing.T) {
	fixture := newConfigFixture(t)
	seeded := config.Default()
	seeded.Retention.RunEventDays = 9
	fixture.seed(t, seeded)

	partial := fullSettings(nil)
	delete(partial, "retention.run_event_days")

	recorder := fixture.put(t, fixture.etag(t), partial)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	body := assertErrorShape(t, recorder, CodeInvalidArgument)
	if body.Field != "retention.run_event_days" {
		t.Fatalf("field = %q, want the missing setting", body.Field)
	}

	content, err := os.ReadFile(fixture.path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), "run_event_days = 9") {
		t.Errorf("the omitted setting was reset:\n%s", content)
	}
}

func TestConfigWriteRejectsBadInput(t *testing.T) {
	tests := []struct {
		name      string
		settings  map[string]string
		wantField string
	}{
		{
			name:      "unknown key",
			settings:  fullSettings(map[string]string{"server.port": "8080"}),
			wantField: "settings",
		},
		{
			name:      "non loopback address",
			settings:  fullSettings(map[string]string{"server.addr": "0.0.0.0:8080"}),
			wantField: "server.addr",
		},
		{
			name:      "unknown log level",
			settings:  fullSettings(map[string]string{"log.level": "loud"}),
			wantField: "log.level",
		},
		{
			name:      "non numeric limit",
			settings:  fullSettings(map[string]string{"capture.max_body_bytes": "big"}),
			wantField: "capture.max_body_bytes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newConfigFixture(t)
			fixture.seed(t, config.Default())
			before := fixture.etag(t)

			recorder := fixture.put(t, before, test.settings)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			body := assertErrorShape(t, recorder, CodeInvalidArgument)
			if body.Field != test.wantField {
				t.Errorf("field = %q, want %q", body.Field, test.wantField)
			}
			if after := fixture.etag(t); after != before {
				t.Error("a rejected write modified the file")
			}
		})
	}
}

func TestConfigWriteRejectsMalformedBody(t *testing.T) {
	fixture := newConfigFixture(t)
	fixture.seed(t, config.Default())

	recorder := fixture.putRaw(t, fixture.etag(t), `{"settings": "not an array"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	assertErrorShape(t, recorder, CodeInvalidArgument)
}

func TestConfigWriteRejectsOversizedBody(t *testing.T) {
	fixture := newConfigFixture(t)
	fixture.seed(t, config.Default())

	oversized := `{"settings":[{"key":"log.level","value":"` +
		strings.Repeat("x", maxConfigBody) + `"}]}`
	recorder := fixture.putRaw(t, fixture.etag(t), oversized)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
	assertErrorShape(t, recorder, CodePayloadTooLarge)
}

// A write lands in the file, but an environment override still wins at runtime.
// The response must say so instead of implying the change took effect.
func TestConfigWriteReportsEnvironmentOverride(t *testing.T) {
	fixture := newConfigFixture(t)
	fixture.seed(t, config.Default())
	fixture.env["MUXIO_LOG_LEVEL"] = "error"

	recorder := fixture.put(t, fixture.etag(t), fullSettings(map[string]string{
		"log.level": "debug",
	}))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}

	var view struct {
		Settings []configSettingView `json:"settings"`
	}
	decode(t, recorder, &view)

	for _, setting := range view.Settings {
		if setting.Key != "log.level" {
			continue
		}
		if setting.Origin != string(config.FromEnv) {
			t.Errorf("origin = %q, want env", setting.Origin)
		}
		if setting.Value != "error" {
			t.Errorf("value = %q, want the still-winning override", setting.Value)
		}
	}

	// The file did receive the write, even though it is not effective yet.
	content, err := os.ReadFile(fixture.path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), `level = "debug"`) {
		t.Errorf("the write did not reach the file:\n%s", content)
	}
}

func TestConfigMethodNotAllowedListsBothMethods(t *testing.T) {
	fixture := newConfigFixture(t)

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/config", nil)
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if allow := recorder.Header().Get("Allow"); allow != "GET, PUT" {
		t.Fatalf("Allow = %q, want %q", allow, "GET, PUT")
	}
}
