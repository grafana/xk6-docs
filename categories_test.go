package docs

import "testing"

func TestIsCategory(t *testing.T) {
	t.Parallel()

	for _, cat := range docCategories() {
		if !isCategory(cat.Name) {
			t.Errorf("isCategory(%q) = false, want true", cat.Name)
		}
	}

	unknown := []string{"get-started", "set-up", "", "foo"}
	for _, name := range unknown {
		if isCategory(name) {
			t.Errorf("isCategory(%q) = true, want false", name)
		}
	}
}

func TestIsIncludedDocsPath(t *testing.T) {
	t.Parallel()

	type tc struct {
		name string
		path string
		want bool
	}

	cats := docCategories()
	cat := cats[0].Name // use for normalization tests

	tests := []tc{
		// Unknown categories are excluded.
		{"get-started excluded", "get-started/welcome", false},
		{"extensions included", "extensions/overview", true},
		{"set-up excluded", "set-up/install", false},
		{"empty path excluded", "", false},

		// Leading/trailing slash normalization.
		{"leading slash", "/" + cat + "/foo", true},
		{"trailing slash", cat + "/bar/", true},
		{"both slashes", "/" + cat + "/baz/", true},

		// Backslash normalization (Windows-style paths).
		{"backslash path", cat + `\foo`, true},
	}

	// Generate cases from docCategories().
	for _, c := range cats {
		if len(c.Subcategories) == 0 {
			tests = append(tests,
				tc{c.Name + " subpath", c.Name + "/something", true},
				tc{"bare " + c.Name, c.Name, true},
			)
			continue
		}
		tests = append(tests,
			tc{"bare " + c.Name + " excluded", c.Name, false},
		)
		for _, sub := range c.Subcategories {
			p := c.Name + "/" + sub
			tests = append(tests,
				tc{p, p, true},
				tc{p + "/child", p + "/child", true},
				tc{c.Name + "/" + sub + ".md", c.Name + "/" + sub + ".md", true},
				tc{p + "/_index.md", p + "/_index.md", true},
				tc{c.Name + "/not-" + sub + " excluded", c.Name + "/not-" + sub, false},
				tc{p + "-old excluded", c.Name + "/" + sub + "-old", false},
				tc{p + "_v2 excluded", c.Name + "/" + sub + "_v2", false},
			)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsIncludedDocsPath(tt.path)
			if got != tt.want {
				t.Errorf("IsIncludedDocsPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
