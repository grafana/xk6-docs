package docs

import (
	"iter"
	"sort"
	"strings"
)

func (idx *Index) ensureMaps() {
	idx.initOnce.Do(func() {
		bySlug := make(map[string]*Section, len(idx.Sections))
		for i := range idx.Sections {
			bySlug[idx.Sections[i].Slug] = &idx.Sections[i]
		}

		byAlias := make(map[string]*Section)
		for i := range idx.Sections {
			sec := &idx.Sections[i]
			for _, a := range sec.Aliases {
				// Exact slug always wins over alias.
				if _, taken := bySlug[a]; taken {
					continue
				}
				if _, dup := byAlias[a]; dup {
					continue
				}
				byAlias[a] = sec
			}
		}

		idx.byAlias = byAlias
		idx.bySlug = bySlug
	})
}

// Lookup returns the section identified by slug. The lookup is
// case-insensitive and alias-aware: a slug match wins over an alias match.
func (idx *Index) Lookup(slug string) (*Section, bool) {
	idx.ensureMaps()
	key := strings.ToLower(slug)
	if sec, ok := idx.bySlug[key]; ok {
		return sec, true
	}
	if sec, ok := idx.byAlias[key]; ok {
		return sec, true
	}
	return nil, false
}

// Children returns the direct child sections of slug in their stored order.
// The prepare tool sorts children by weight at build time.
// Returns nil if slug is unknown.
func (idx *Index) Children(slug string) []*Section {
	idx.ensureMaps()
	parent, ok := idx.bySlug[slug]
	if !ok {
		return nil
	}
	out := make([]*Section, 0, len(parent.Children))
	for _, child := range parent.Children {
		if c, ok := idx.bySlug[child]; ok {
			out = append(out, c)
		}
	}
	return out
}

// normalize strips separators (dashes, spaces, slashes), then lowercases.
// This enables fuzzy matching where "k6 http get", "k6-http-get",
// and "k6/http/get" all compare equal.
func normalize(s string) string {
	return strings.ToLower(strings.NewReplacer("-", "", " ", "", "/", "").Replace(s))
}

// Search returns sections whose title, description, slug, or body (via
// readContent) contain term. Matching is case-insensitive on title and
// description, and normalized fuzzy (ignoring spaces, dashes, slashes) on
// title, description, and slug. If readContent is non-nil, the body returned
// by readContent(slug) is matched the same two ways. Results preserve the
// order of idx.Sections and contain no duplicates. Returns nil for an empty
// term.
func (idx *Index) Search(term string, readContent func(slug string) string) []*Section {
	if term == "" {
		return nil
	}

	lower := strings.ToLower(term)
	normTerm := normalize(term)
	var results []*Section

	for i := range idx.Sections {
		sec := &idx.Sections[i]

		if strings.Contains(strings.ToLower(sec.Title), lower) ||
			strings.Contains(strings.ToLower(sec.Description), lower) {
			results = append(results, sec)
			continue
		}

		if strings.Contains(normalize(sec.Title), normTerm) ||
			strings.Contains(normalize(sec.Description), normTerm) ||
			strings.Contains(normalize(sec.Slug), normTerm) {
			results = append(results, sec)
			continue
		}

		if readContent != nil {
			body := readContent(sec.Slug)
			if body == "" {
				continue
			}
			if strings.Contains(strings.ToLower(body), lower) ||
				strings.Contains(normalize(body), normTerm) {
				results = append(results, sec)
			}
		}
	}

	return results
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

// ByCategory returns sections whose Category equals category,
// preserving the order in idx.Sections.
func (idx *Index) ByCategory(category string) []*Section {
	var out []*Section
	for i := range idx.Sections {
		sec := &idx.Sections[i]
		if sec.Category == category {
			out = append(out, sec)
		}
	}
	return out
}

// Tree iterates the section tree rooted at rootSlug, depth-first.
// When rootSlug is empty, iteration starts at the top-level sections.
// Otherwise iteration starts at the children of rootSlug; the root itself
// is not yielded. Each yielded *Tree has its Children populated only down
// to the requested depth. If depth < 1 or rootSlug is unknown, nothing is
// yielded.
func (idx *Index) Tree(rootSlug string, depth int) iter.Seq2[int, *Tree] {
	return func(yield func(int, *Tree) bool) {
		if depth < 1 {
			return
		}

		var roots []*Section
		if rootSlug == "" {
			roots = idx.TopLevel()
		} else {
			idx.ensureMaps()
			if _, ok := idx.bySlug[rootSlug]; !ok {
				return
			}
			roots = idx.Children(rootSlug)
		}

		sort.SliceStable(roots, func(i, j int) bool {
			if roots[i].Weight != roots[j].Weight {
				return roots[i].Weight < roots[j].Weight
			}
			return roots[i].Slug < roots[j].Slug
		})

		for _, sec := range roots {
			t := idx.buildTree(sec, depth)
			if !walkTree(t, 0, yield) {
				return
			}
		}
	}
}

func (idx *Index) buildTree(sec *Section, depth int) *Tree {
	t := &Tree{Section: sec}
	if depth <= 1 {
		return t
	}
	for _, child := range idx.Children(sec.Slug) {
		t.Children = append(t.Children, idx.buildTree(child, depth-1))
	}
	return t
}

func walkTree(t *Tree, level int, yield func(int, *Tree) bool) bool {
	if !yield(level, t) {
		return false
	}
	for _, child := range t.Children {
		if !walkTree(child, level+1, yield) {
			return false
		}
	}
	return true
}
