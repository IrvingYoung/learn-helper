package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// With a SetSPAFS-injected fs.FS, getDistIndexHTML returns its content
// without touching disk.
func TestGetDistIndexHTML_FromSpaFS(t *testing.T) {
	const want = `<!doctype html><html><body id="root">hi</body></html>`
	prev := spaFS
	t.Cleanup(func() { spaFS = prev })

	SetSPAFS(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(want)},
	})
	// Unset LH_SPA_DIST so a stray value doesn't mask failures.
	t.Setenv("LH_SPA_DIST", "")

	got, err := getDistIndexHTML()
	if err != nil {
		t.Fatalf("getDistIndexHTML: %v", err)
	}
	if string(got) != want {
		t.Errorf("got %q, want %q", string(got), want)
	}
}

// Without spaFS set, getDistIndexHTML falls back to LH_SPA_DIST.
func TestGetDistIndexHTML_DiskFallback_LHSPADist(t *testing.T) {
	dir := t.TempDir()
	idx := filepath.Join(dir, "index.html")
	if err := os.WriteFile(idx, []byte("<html>disk</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	prev := spaFS
	t.Cleanup(func() { spaFS = prev })
	spaFS = nil
	t.Setenv("LH_SPA_DIST", dir)

	got, err := getDistIndexHTML()
	if err != nil {
		t.Fatalf("getDistIndexHTML: %v", err)
	}
	if !strings.Contains(string(got), "disk") {
		t.Errorf("got %q, expected disk content", string(got))
	}
}

// With both spaFS and disk empty, returns os.ErrNotExist.
func TestGetDistIndexHTML_Neither(t *testing.T) {
	prev := spaFS
	t.Cleanup(func() { spaFS = prev })
	spaFS = nil

	// Point LH_SPA_DIST at a dir that does not have index.html.
	dir := t.TempDir()
	t.Setenv("LH_SPA_DIST", dir)
	// And we can't unset CWD, but the candidates after LH_SPA_DIST are
	// relative to the test runner — they will also miss, since the
	// runner's frontend/dist is not built in unit tests.

	if _, err := getDistIndexHTML(); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// SetSPAFS(nil) is a no-op.
func TestSetSPAFS_NilIsNoop(t *testing.T) {
	prev := spaFS
	t.Cleanup(func() { spaFS = prev })
	spaFS = fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}}

	SetSPAFS(nil)
	if spaFS == nil {
		t.Fatal("SetSPAFS(nil) should not clear an existing fs")
	}
}
