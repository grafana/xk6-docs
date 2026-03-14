package docs

import "strings"

// Category holds a top-level documentation category with its inclusion rule.
type Category struct {
	Name string
	// Subcategories lists the only allowed second-level segments when set.
	// An empty slice means all subpaths are included.
	Subcategories []string
}

func docCategories() []Category {
	return []Category{
		{Name: "javascript-api"},
		{Name: "using-k6"},
		{Name: "using-k6-browser"},
		{Name: "testing-guides"},
		{Name: "examples"},
		{Name: "results-output"},
		{Name: "extensions"},
		{Name: "set-up"},
		{Name: "get-started"},
		{Name: "release-notes"},
		{Name: "grafana-cloud-k6"},
		{Name: "k6-studio"},
		{Name: "reference"},
	}
}

func isCategory(name string) bool {
	for _, c := range docCategories() {
		if c.Name == name {
			return true
		}
	}
	return false
}

// IsIncludedDocsPath reports whether a documentation path should be included
// in the doc bundle. It normalizes the path and checks against the canonical
// category list.
func IsIncludedDocsPath(path string) bool {
	path = strings.ReplaceAll(path, `\`, "/")
	path = strings.Trim(path, "/")
	if path == "" {
		return false
	}

	cat, rest, _ := strings.Cut(path, "/")

	for _, c := range docCategories() {
		if c.Name != cat {
			continue
		}
		if len(c.Subcategories) == 0 {
			return true
		}
		// Category has restricted subcategories — bare category is excluded.
		if rest == "" {
			return false
		}
		seg, _, _ := strings.Cut(rest, "/")
		for _, sub := range c.Subcategories {
			if seg == sub || seg == sub+".md" {
				return true
			}
		}
		return false
	}
	return false
}
