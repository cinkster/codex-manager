package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// NewShareServer serves only exact filenames from the share directory.
func NewShareServer(shareDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || strings.Contains(path, "/") || strings.Contains(path, "\\") || strings.Contains(path, "..") {
			http.NotFound(w, r)
			return
		}

		target := filepath.Join(shareDir, path)
		info, err := os.Stat(target)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, target)
	})
}
