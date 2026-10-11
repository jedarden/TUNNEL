package embed

import (
	"bytes"
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

// HasRealFrontend reports whether the embedded dist holds a real production
// build of the React app rather than the placeholder page written by
// `make build-dev`. A real Vite build always mounts the app at
// <div id="root"> and references hashed assets under /assets/.
func HasRealFrontend() bool {
	index, err := fs.ReadFile(staticFS, "dist/index.html")
	if err != nil {
		return false
	}
	if !bytes.Contains(index, []byte(`id="root"`)) || !bytes.Contains(index, []byte("/assets/")) {
		return false
	}
	assets, err := fs.ReadDir(staticFS, "dist/assets")
	if err != nil {
		return false
	}
	for _, asset := range assets {
		if !asset.IsDir() {
			return true
		}
	}
	return false
}
