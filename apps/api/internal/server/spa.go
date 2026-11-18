package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
)

// spaHandler serves the embedded web app. Unknown paths fall back to index.html so Vue Router can
// handle them. Unknown /api/ paths return JSON 404 instead. meta may name the page of a request, so a
// link pasted in a chat app shows what it points to.
func spaHandler(dist fs.FS, meta func(*http.Request) *pageMeta) http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	index, indexErr := fs.ReadFile(sub, "index.html")
	files := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "Route not found"})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if indexErr != nil {
			http.Error(w, "Web app not built. Run `npm run build` at the repo root, or use the Vite dev server.", http.StatusNotFound)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" {
			if f, err := sub.Open(name); err == nil {
				st, statErr := f.Stat()
				f.Close()
				if statErr == nil && !st.IsDir() {
					files.ServeHTTP(w, r)
					return
				}
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		page := index
		if meta != nil {
			page = withMeta(index, meta(r))
		}
		_, _ = w.Write(page)
	})
}
