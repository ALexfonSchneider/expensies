// Package spa serves the embedded single-page frontend.
package spa

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const notBuilt = `<!doctype html><meta charset="utf-8"><title>expenses</title>
<body style="font-family:system-ui;padding:2rem;max-width:40rem">
<h1>Frontend is not built</h1>
<p>Run <code>npm install &amp;&amp; npm run build</code> inside <code>web/</code>,
then rebuild the Go binary. The JSON API under <code>/api/v1</code> works regardless.</p>`

// New serves files from the "dist" directory of the given filesystem with
// history-API fallback: a path that does not match a file returns
// index.html so client-side routes survive a browser refresh. When dist has
// no index.html (frontend not built) every path returns a short
// explanation instead of a bare 404.
func New(dist fs.FS) (http.Handler, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("spa: %w", err)
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, notBuilt)
		}), nil
	}

	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p != "" && p != "index.html" {
			if f, err := sub.Open(p); err == nil {
				// Only probing existence; FileServer reopens the file and an
				// embedded read-only file cannot fail to close.
				_ = f.Close()
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		// A write failure means the browser went away; nothing to do.
		_, _ = w.Write(index)
	}), nil
}
