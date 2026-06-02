package skills

import "embed"

//go:embed *.md
var embedFS embed.FS

// EmbedFS returns the embedded filesystem containing all SKILL.md files.
// Used as the default load source; override with os.DirFS(LH_SKILLS_DIR) in dev.
func EmbedFS() embed.FS {
	return embedFS
}
