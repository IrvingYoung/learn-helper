package skills

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadFromFS_HappyPath(t *testing.T) {
	fsys := fstest.MapFS{
		"explain-page.md": &fstest.MapFile{
			Data: []byte("---\n" +
				"name: explain-page\n" +
				"description: 解释当前页面的核心概念\n" +
				"license: MIT\n" +
				"---\n\n" +
				"你正在用通俗解释模式。\n"),
		},
	}
	r, err := LoadFromFS(fsys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := r.Get("explain-page")
	if !ok {
		t.Fatal("expected explain-page to be registered")
	}
	if s.Description != "解释当前页面的核心概念" {
		t.Errorf("Description = %q", s.Description)
	}
	if !strings.Contains(s.Body, "通俗解释模式") {
		t.Errorf("Body missing expected text: %q", s.Body)
	}
	if got, ok := s.Extra["license"]; !ok || got != "MIT" {
		t.Errorf("Extra[license] = %v", got)
	}
	if !strings.HasSuffix(s.SourcePath, "explain-page.md") {
		t.Errorf("SourcePath = %q", s.SourcePath)
	}
}

func TestLoadFromFS_MissingName(t *testing.T) {
	fsys := fstest.MapFS{
		"bad.md": &fstest.MapFile{Data: []byte("---\ndescription: hi\n---\nbody")},
	}
	_, err := LoadFromFS(fsys)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Errorf("error should mention file path: %v", err)
	}
}

func TestLoadFromFS_MissingDescription(t *testing.T) {
	fsys := fstest.MapFS{
		"bad.md": &fstest.MapFile{Data: []byte("---\nname: x\n---\nbody")},
	}
	_, err := LoadFromFS(fsys)
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("expected error mentioning description, got %v", err)
	}
}

func TestLoadFromFS_DuplicateName(t *testing.T) {
	fsys := fstest.MapFS{
		"a.md": &fstest.MapFile{Data: []byte("---\nname: foo\ndescription: A\n---\nx")},
		"b.md": &fstest.MapFile{Data: []byte("---\nname: foo\ndescription: B\n---\ny")},
	}
	_, err := LoadFromFS(fsys)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestLoadFromFS_EmptyBodyIsOK(t *testing.T) {
	fsys := fstest.MapFS{
		"empty.md": &fstest.MapFile{Data: []byte("---\nname: e\ndescription: E\n---\n")},
	}
	r, err := LoadFromFS(fsys)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	s, _ := r.Get("e")
	if s.Body != "" {
		t.Errorf("expected empty body, got %q", s.Body)
	}
}

func TestLoadFromFS_OrderingIsAlphabetical(t *testing.T) {
	fsys := fstest.MapFS{
		"z.md": &fstest.MapFile{Data: []byte("---\nname: zed\ndescription: Z\n---\n")},
		"a.md": &fstest.MapFile{Data: []byte("---\nname: aardvark\ndescription: A\n---\n")},
	}
	r, _ := LoadFromFS(fsys)
	got := r.List()
	if len(got) != 2 || got[0].Name != "aardvark" || got[1].Name != "zed" {
		t.Errorf("unexpected order: %+v", got)
	}
}

func TestLoadFromFS_BodyMayContainFences(t *testing.T) {
	fsys := fstest.MapFS{
		"x.md": &fstest.MapFile{Data: []byte("---\nname: x\ndescription: X\n---\n" +
			"body line 1\n---\nbody line 2\n")},
	}
	r, err := LoadFromFS(fsys)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	s, _ := r.Get("x")
	want := "body line 1\n---\nbody line 2\n"
	if s.Body != want {
		t.Errorf("body = %q, want %q", s.Body, want)
	}
}
