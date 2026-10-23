// Package web menyimpan hasil build frontend (web/dist) yang di-embed ke binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS mengembalikan filesystem hasil build frontend.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
