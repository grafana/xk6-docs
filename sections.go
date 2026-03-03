package docs

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"go.k6.io/k6/lib/fsext"
)

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
}

// Index holds all sections and provides fast lookup by slug.
type Index struct {
	Version  string    `json:"version"`
	Sections []Section `json:"sections"`
	bySlug   map[string]*Section
}

// LoadIndex reads sections.json from dir and returns a populated Index.
func LoadIndex(afs fsext.Fs, dir string) (*Index, error) {
	data, err := fsext.ReadFile(afs, filepath.Join(dir, "sections.json"))
	if err != nil {
		return nil, fmt.Errorf("load index %s: %w", dir, err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index %s: %w", dir, err)
	}

	idx.bySlug = make(map[string]*Section, len(idx.Sections))
	for i := range idx.Sections {
		idx.bySlug[idx.Sections[i].Slug] = &idx.Sections[i]
	}

	return &idx, nil
}

// Lookup returns the section with the given slug in O(1) time.
// The lookup is case-insensitive.
func (idx *Index) Lookup(slug string) (*Section, bool) {
	sec, ok := idx.bySlug[strings.ToLower(slug)]
	return sec, ok
}

// normalize strips separators (dashes, spaces, slashes), then lowercases.
// This enables fuzzy matching where "close context", "close-context",
// "close/context", and "closecontext" all compare equal.
func normalize(s string) string {
	return strings.ToLower(strings.NewReplacer("-", "", " ", "", "/", "").Replace(s))
}

// Search returns sections whose title, description, slug, or body (via readContent)
// contain term as a case-insensitive substring. If readContent is nil, only
// title, description, and slug are checked.
//
// In addition to exact case-insensitive matching, Search performs normalized
// matching that ignores spaces and dashes so that e.g. "close context" matches
// "closecontext".
func (idx *Index) Search(term string, readContent func(slug string) string) []*Section {
	if term == "" {
		return nil
	}

	lower := strings.ToLower(term)
	normTerm := normalize(term)
	var results []*Section

	for i := range idx.Sections {
		sec := &idx.Sections[i]

		// Exact case-insensitive match on title or description.
		if strings.Contains(strings.ToLower(sec.Title), lower) ||
			strings.Contains(strings.ToLower(sec.Description), lower) {
			results = append(results, sec)
			continue
		}

		// Normalized (fuzzy) match: ignore spaces and dashes.
		if strings.Contains(normalize(sec.Title), normTerm) ||
			strings.Contains(normalize(sec.Description), normTerm) ||
			strings.Contains(normalize(sec.Slug), normTerm) {
			results = append(results, sec)
			continue
		}

		if readContent != nil {
			body := readContent(sec.Slug)
			if body != "" {
				if strings.Contains(strings.ToLower(body), lower) ||
					strings.Contains(normalize(body), normTerm) {
					results = append(results, sec)
				}
			}
		}
	}

	return results
}

// Children returns the child sections of the given slug, sorted by weight.
// Returns nil if the slug is not found.
func (idx *Index) Children(slug string) []*Section {
	parent, ok := idx.bySlug[slug]
	if !ok {
		return nil
	}

	children := make([]*Section, 0, len(parent.Children))
	for _, childSlug := range parent.Children {
		if child, ok := idx.bySlug[childSlug]; ok {
			children = append(children, child)
		}
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].Weight < children[j].Weight
	})

	return children
}

// TopLevel returns sections where Category == Slug (top-level indices),
// sorted by weight.
func (idx *Index) TopLevel() []*Section {
	var top []*Section
	for i := range idx.Sections {
		sec := &idx.Sections[i]
		if sec.Category == sec.Slug {
			top = append(top, sec)
		}
	}

	sort.Slice(top, func(i, j int) bool {
		return top[i].Weight < top[j].Weight
	})

	return top
}
