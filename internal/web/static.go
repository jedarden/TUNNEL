// Package web wires the embedded React frontend into the Fiber app used by
// cmd/tunnel, so the serving path is shared with the smoke tests instead of
// being duplicated in the command.
package web

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	embeddedfs "github.com/jedarden/tunnel/internal/web/embed"
)

// ErrFrontendNotEmbedded is returned by MountFrontend when the embedded
// filesystem holds no index.html — the binary was built without the React
// bundle (see make build-frontend / make build-dev).
var ErrFrontendNotEmbedded = errors.New("web UI not embedded: dist/index.html missing (run make build-frontend)")

// MountFrontend serves the embedded React application at "/" with an SPA
// fallback: paths that match no file (e.g. /settings/providers) are answered
// with index.html so client-side routing keeps working on deep links.
func MountFrontend(app *fiber.App) error {
	staticFS, err := embeddedfs.GetFS()
	if err != nil {
		return err
	}
	if _, err := fs.Stat(staticFS, "index.html"); err != nil {
		return ErrFrontendNotEmbedded
	}

	app.Use("/", filesystem.New(filesystem.Config{
		Root:         http.FS(staticFS),
		Browse:       false,
		Index:        "index.html",
		NotFoundFile: "index.html", // SPA fallback
	}))
	return nil
}
