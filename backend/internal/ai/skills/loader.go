package skills

import (
	"bytes"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFromFS walks the given fs.FS for *.md files, parses each as a
// SKILL.md (YAML frontmatter + markdown body), and returns a populated
// Registry. Errors are aggregated: if any file is invalid, LoadFromFS
// returns the aggregated error and a nil registry.
func LoadFromFS(fsys fs.FS) (*Registry, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var mdFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") {
			mdFiles = append(mdFiles, e.Name())
		}
	}
	sort.Strings(mdFiles)

	r := NewRegistry()
	var errs []string
	for _, name := range mdFiles {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: read: %v", name, err))
			continue
		}
		s, err := parseOne(name, raw)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !r.Add(s) {
			errs = append(errs, fmt.Sprintf("%s: duplicate name %q", name, s.Name))
			continue
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("skill loader: %d error(s):\n  %s",
			len(errs), strings.Join(errs, "\n  "))
	}
	return r, nil
}

// parseOne splits frontmatter from body and decodes frontmatter as YAML.
// Unknown fields land in Extra so we don't reject Claude Code-compatible
// fields like license/compatibility/metadata.
func parseOne(sourcePath string, raw []byte) (*Skill, error) {
	front, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, err
	}
	if len(front) == 0 {
		return nil, fmt.Errorf("missing frontmatter (expected `---` delimited YAML at top)")
	}

	// Decode into a generic map so unknown fields are preserved.
	var generic map[string]any
	if err := yaml.Unmarshal(front, &generic); err != nil {
		return nil, fmt.Errorf("parse frontmatter yaml: %w", err)
	}

	name, _ := generic["name"].(string)
	desc, _ := generic["description"].(string)
	if name == "" {
		return nil, fmt.Errorf("frontmatter missing required field `name`")
	}
	if desc == "" {
		return nil, fmt.Errorf("frontmatter missing required field `description`")
	}

	// Normalize metadata sub-map if present.
	if md, ok := generic["metadata"].(map[string]any); ok {
		generic["metadata"] = md
	}

	return &Skill{
		Name:        name,
		Description: desc,
		Body:        string(body),
		Extra:       generic,
		SourcePath:  sourcePath,
	}, nil
}

// splitFrontmatter returns the bytes between the first pair of `---` lines
// and everything after. If no frontmatter is present, front is empty.
func splitFrontmatter(raw []byte) (front, body []byte, err error) {
	const sep = "---"
	// Allow optional leading BOM / whitespace.
	trimmed := bytes.TrimLeft(raw, "\xef\xbb\xbf \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte(sep)) {
		return nil, raw, nil
	}
	rest := trimmed[len(sep):]
	// Must be followed by a newline.
	if len(rest) == 0 || rest[0] != '\n' {
		return nil, raw, fmt.Errorf("frontmatter opening `---` must be followed by newline")
	}
	rest = rest[1:]

	// Find the closing --- on its own line.
	lines := bytes.Split(rest, []byte("\n"))
	endIdx := -1
	for i, line := range lines {
		trimmedLine := bytes.TrimSpace(line)
		if bytes.Equal(trimmedLine, []byte(sep)) {
			endIdx = i
			break
		}
	}
	if endIdx < 0 {
		return nil, raw, fmt.Errorf("frontmatter not closed (no closing `---`)")
	}

	front = bytes.Join(lines[:endIdx], []byte("\n"))
	// Body is everything after the closing --- line.
	remaining := lines[endIdx+1:]
	body = bytes.TrimLeft(bytes.Join(remaining, []byte("\n")), "\r\n")
	return front, body, nil
}
