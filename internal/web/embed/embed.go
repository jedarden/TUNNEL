package embed

import (
	"embed"
	"io/fs"
)

// StaticFS embeds the built React application
// The all:dist directive includes all files in the dist directory
//
//go:embed all:dist
var staticFS embed.FS

// GetFS returns the embedded filesystem rooted at dist/
// This removes the "dist" prefix from paths when serving files
func GetFS() (fs.FS, error) {
	return fs.Sub(staticFS, "dist")
}

// MustGetFS is like GetFS but panics on error
func MustGetFS() fs.FS {
	fsys, err := GetFS()
	if err != nil {
		panic(err)
	}
	return fsys
}
