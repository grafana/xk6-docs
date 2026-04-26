// Package docs provides types and operations for k6 documentation.
// It supports per-version documentation bundles with lazy loading,
// HTTP+cache or embedded FS backends, and content transforms.
package docs

import "sync"

// Section represents a single documentation section.
type Section struct {
	Slug        string   `json:"slug"`
	RelPath     string   `json:"rel_path"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Weight      int      `json:"weight"`
	Category    string   `json:"category"`
	Children    []string `json:"children"`
	IsIndex     bool     `json:"is_index"`
	Aliases     []string `json:"aliases,omitempty"`
}

// Index holds all sections for a single documentation version.
type Index struct {
	Version  string    `json:"version"`
	Sections []Section `json:"sections"`

	initOnce sync.Once
	bySlug   map[string]*Section
	byAlias  map[string]*Section
}

// Tree is a depth-limited node in a section tree.
type Tree struct {
	*Section
	Children []*Tree
}
