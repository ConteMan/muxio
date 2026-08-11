package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPanelIsEmbedded(t *testing.T) {
	// A binary without the panel is a release defect, so this is not skipped.
	if !Available() {
		t.Fatal("no panel is embedded; run `npm --prefix web run build`")
	}
}

func TestServesIndex(t *testing.T) {
	recorder := get(t, "/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "<div id=\"root\">") {
		t.Fatalf("body does not look like the panel: %q", truncate(recorder.Body.String()))
	}
	// index.html must be revalidated or an upgraded binary keeps serving the
	// previous build's markup.
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cache)
	}
}

// Client-side routes are not files; they must land on the panel, not a 404.
func TestUnknownPathFallsBackToIndex(t *testing.T) {
	for _, path := range []string{"/runs", "/runs/12", "/settings", "/deeply/nested/route"} {
		t.Run(path, func(t *testing.T) {
			recorder := get(t, path)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if !strings.Contains(recorder.Body.String(), "<div id=\"root\">") {
				t.Fatal("fallback did not serve the panel")
			}
		})
	}
}

// Hashed asset names are immutable, so they may be cached indefinitely.
func TestHashedAssetsAreCacheable(t *testing.T) {
	index := get(t, "/").Body.String()

	start := strings.Index(index, "/assets/")
	if start < 0 {
		t.Skip("the index references no hashed assets")
	}
	end := strings.IndexAny(index[start:], `"'`)
	if end < 0 {
		t.Fatal("could not parse the asset reference")
	}
	assetPath := index[start : start+end]

	recorder := get(t, assetPath)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d for %s", recorder.Code, assetPath)
	}
	if cache := recorder.Header().Get("Cache-Control"); !strings.Contains(cache, "immutable") {
		t.Errorf("Cache-Control = %q, want it to allow long caching", cache)
	}
}

// Traversal must not escape the embedded tree.
func TestPathTraversalIsContained(t *testing.T) {
	for _, path := range []string{"/../go.mod", "/assets/../../go.mod"} {
		recorder := get(t, path)
		if body := recorder.Body.String(); strings.Contains(body, "module github.com") {
			t.Fatalf("%s escaped the embedded assets", path)
		}
	}
}

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func truncate(value string) string {
	if len(value) <= 200 {
		return value
	}
	return value[:200] + "…"
}
