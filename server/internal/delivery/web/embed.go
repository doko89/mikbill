// Package web menyajikan frontend React yang di-embed ke binary.
package web

import (
	"net/http"
	"strings"

	rootweb "github.com/mikbil/mikbil/web"
)

// Handler mengembalikan handler file statis dengan fallback index.html (SPA).
func Handler() http.Handler {
	sub, err := rootweb.FS()
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend belum dibangun", http.StatusServiceUnavailable)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := sub.Open(path); err == nil {
				stat, serr := f.Stat()
				f.Close()
				if serr == nil && !stat.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
