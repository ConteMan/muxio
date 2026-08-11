// Package webui serves the panel that ships inside the binary.
//
// The assets are built from web/ and committed to this directory (ADR-007), so
// `go build` never needs Node.
package webui

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:assets
var assets embed.FS

// staticModTime is the timestamp reported for embedded content. Embedded files
// carry no modification time, and a fixed value keeps responses deterministic.
var staticModTime = time.Unix(0, 0)

// indexFile is the single page every unmatched route falls back to.
const indexFile = "index.html"

// Available reports whether a panel was actually embedded. A source tree whose
// assets were never built yields an index-less directory, and saying so beats
// serving a blank page.
func Available() bool {
	_, err := fs.Stat(assetsFS(), indexFile)
	return err == nil
}

func assetsFS() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		// The embed directive is a compile-time constant, so this cannot happen.
		panic(err)
	}
	return sub
}

// Handler serves the panel, falling back to index.html so client-side routes
// resolve. It never serves API paths: the caller routes those first.
func Handler() http.Handler {
	files := assetsFS()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !Available() {
			http.Error(w,
				"the web panel was not built into this binary; run `npm --prefix web run build`",
				http.StatusNotImplemented)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = indexFile
		}

		file, err := files.Open(name)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "could not read the panel", http.StatusInternalServerError)
				return
			}
			// An unknown path is a client-side route, not a missing file.
			serveIndex(w, r, files)
			return
		}
		defer func() { _ = file.Close() }()

		info, err := file.Stat()
		if err != nil || info.IsDir() {
			serveIndex(w, r, files)
			return
		}

		// Asset filenames carry a content hash, so they are safe to cache, but
		// index.html must be revalidated or an upgraded binary keeps serving the
		// previous build's markup.
		if name == indexFile {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		readSeeker, ok := file.(interface {
			Read([]byte) (int, error)
			Seek(int64, int) (int64, error)
		})
		if !ok {
			http.Error(w, "could not read the panel", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), readSeeker)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, files fs.FS) {
	content, err := fs.ReadFile(files, indexFile)
	if err != nil {
		http.Error(w, "could not read the panel", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, indexFile, staticModTime, strings.NewReader(string(content)))
}
