// Package webui embeds the built admin frontend (admin/, built via Vite)
// so a single Go binary can serve both the device/admin API and the
// browser UI, with no separate static-file deployment step.
//
// dist/ starts out with only a placeholder .gitkeep committed to the repo,
// so a fresh checkout still compiles (go:embed requires its pattern to
// match at least one file) even before the frontend has ever been built.
// Run the repo's build script (build.ps1 / build.sh), or just
// `cd admin && npm run build` — its outDir is configured to emit straight
// into this directory — then rebuild the server to bundle the real app.
package webui

import (
	"embed"
	"io/fs"
)

// all: is required to include dotfiles like the placeholder .gitkeep;
// without it, embed silently skips anything starting with "." or "_".
//
//go:embed all:dist
var distFS embed.FS

// FS returns the embedded frontend build rooted at dist/ (so "index.html"
// and "assets/..." resolve directly, without a "dist/" prefix), and
// whether it actually contains a built app (index.html present) as
// opposed to just the placeholder from a checkout that hasn't run the
// frontend build yet.
func FS() (ui fs.FS, built bool, err error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, false, err
	}
	if f, openErr := sub.Open("index.html"); openErr == nil {
		_ = f.Close()
		built = true
	}
	return sub, built, nil
}
