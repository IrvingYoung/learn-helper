package handler

import (
	"io/fs"
	"os"
)

// spaFS holds the embedded SPA's `dist/` directory as an fs.FS. In production
// it is set by main.go via SetSPAFS using a //go:embed all:dist. In tests
// and in dev builds without a populated dist/, it is nil — getDistIndexHTML
// will then fall back to the on-disk candidate paths.
var spaFS fs.FS

// SetSPAFS injects the embedded SPA filesystem. Call once from main() before
// serving requests. Safe no-op if fs is nil.
func SetSPAFS(f fs.FS) {
	if f == nil {
		return
	}
	spaFS = f
}

// getDistIndexHTML returns the SPA's index.html bytes. Order:
//  1. embedded dist (production)
//  2. on-disk candidate paths (dev / override via LH_SPA_DIST)
func getDistIndexHTML() ([]byte, error) {
	if spaFS != nil {
		if b, err := fs.ReadFile(spaFS, "index.html"); err == nil {
			return b, nil
		}
	}
	return readDistIndexFromDisk()
}

// readDistIndexFromDisk is the disk-fallback path. Looks in several
// conventional paths so the binary works from `backend/` (dev) and from
// a deployment dir (prod). Override with the LH_SPA_DIST env var.
//
// Returns an error if no candidate path contains the file. The caller
// (GetShareSSRPage) converts this to a 500 with a useful message.
func readDistIndexFromDisk() ([]byte, error) {
	cwd, _ := os.Getwd()

	candidates := []string{}
	if v := os.Getenv("LH_SPA_DIST"); v != "" {
		candidates = append(candidates, v+"/index.html")
	}
	candidates = append(candidates,
		cwd+"/frontend/dist/index.html",
		cwd+"/../frontend/dist/index.html",
		cwd+"/dist/index.html",
		"./frontend/dist/index.html",
		"../frontend/dist/index.html",
		"./dist/index.html",
	)
	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			return b, nil
		}
	}
	return nil, os.ErrNotExist
}
