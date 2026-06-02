// Package skills loads SKILL.md files into a registry. The markdown format
// is compatible with Claude Code skills: YAML frontmatter (name, description,
// optional extras) + a free-form markdown body.
package skills

import (
	"sort"
	"sync"
)

// Skill is one loaded SKILL.md file.
type Skill struct {
	Name        string
	Description string
	Body        string
	Extra       map[string]any
	SourcePath  string
}

// Registry is a thread-safe collection of skills indexed by name.
type Registry struct {
	mu    sync.RWMutex
	byKey map[string]*Skill
	order []string
}

func NewRegistry() *Registry {
	return &Registry{byKey: map[string]*Skill{}}
}

// Add inserts a skill. Returns false if the name is already registered.
func (r *Registry) Add(s *Skill) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byKey[s.Name]; exists {
		return false
	}
	r.byKey[s.Name] = s
	r.order = append(r.order, s.Name)
	sort.Strings(r.order)
	return true
}

// Get returns the skill with the given name and a bool indicating presence.
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byKey[name]
	return s, ok
}

// List returns all skills sorted by name.
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Skill, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.byKey[n])
	}
	return out
}
