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
