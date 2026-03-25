package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	docs "github.com/grafana/xk6-docs"
	"go.k6.io/k6/lib/fsext"
)

// setupMockDocs creates a minimal mock k6-docs directory structure for testing.
// It returns the in-memory filesystem and root path.
func setupMockDocs(t *testing.T) (fsext.Fs, string) {
	t.Helper()

	afs := fsext.NewMemMapFs()
	root := "/mock-docs"
	versionRoot := filepath.Join(root, "docs", "sources", "k6", "v0.99.x")

	// Create version root _index.md (should be skipped).
	writeFile(t, afs, filepath.Join(versionRoot, "_index.md"), `---
title: 'k6 Documentation'
description: 'The k6 documentation.'
weight: 1
---

# k6 Documentation
`)

	// Create shared content.
	writeFile(t, afs, filepath.Join(versionRoot, "shared", "index.md"), `---
headless: true
---
`)
	writeFile(t, afs, filepath.Join(versionRoot, "shared", "javascript-api", "k6-mod-a.md"), `---
title: 'k6/mod-a shared content'
---

The k6/mod-a module contains functionality for performing requests.

| Method  | Description |
|---------|-------------|
| fn-one  | Issue a first function call. |
`)

	// Create javascript-api category.
	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "_index.md"), `---
title: 'JavaScript API'
description: 'The list of k6 modules natively supported in k6 scripts.'
weight: 03
---

# JavaScript API

The list of k6 modules.

{{< section >}}
`)

	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "k6-mod-a", "_index.md"), `---
title: 'k6/mod-a'
description: 'The k6/mod-a module contains functionality for performing requests.'
weight: 09
---

# k6/mod-a

{{< docs/shared source="k6" lookup="javascript-api/k6-mod-a.md" version="<K6_VERSION>" >}}
`)

	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "k6-mod-a", "fn-one.md"), `---
title: 'fnOne( url, [params] )'
description: 'Issue a first function call.'
weight: 10
---

# fnOne( url, [params] )

Make a first function call.

See the [API docs](https://grafana.com/docs/k6/<K6_VERSION>/javascript-api/k6-mod-a/fn-one).

{{< code >}}

`+"```javascript"+`
import modA from 'k6/mod-a';

export default function () {
  const res = modA.fnOne('https://test.example.com');
}
`+"```"+`

{{< /code >}}
`)

	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "k6-mod-a", "fn-two.md"), `---
title: 'fnTwo( url, [body], [params] )'
description: 'Issue a second function call.'
weight: 20
---

# fnTwo( url, [body], [params] )

Make a second function call.
`)

	// Create alpha category.
	writeFile(t, afs, filepath.Join(versionRoot, "alpha", "_index.md"), `---
title: 'Alpha'
description: 'The alpha section.'
weight: 05
---

# Alpha
`)

	writeFile(t, afs, filepath.Join(versionRoot, "alpha", "topic-two.md"), `---
title: 'TopicTwo'
description: 'TopicTwo items validate boolean conditions.'
weight: 400
---

# TopicTwo

TopicTwo items validate boolean conditions in your test.

{{< admonition type="note" >}}

When an item fails, the script continues.

{{< /admonition >}}
`)

	writeFile(t, afs, filepath.Join(versionRoot, "alpha", "topic-three.md"), `---
title: 'TopicThree'
description: 'TopicThree items are pass/fail criteria.'
weight: 500
---

# TopicThree

TopicThree items are pass/fail criteria for your test metrics.
`)

	// Create lambda/sub-a (should be included).
	writeFile(t, afs, filepath.Join(versionRoot, "lambda", "_index.md"), `---
title: 'Lambda'
description: 'k6 lambda documentation.'
weight: 100
---

# Lambda
`)

	writeFile(t, afs, filepath.Join(versionRoot, "lambda", "sub-a.md"), `---
title: 'SubA'
description: 'Technical terms used in k6.'
weight: 07
---

# SubA

What we talk about when we talk about k6.
`)

	// Create lambda/sub-b.md (should be EXCLUDED — only sub-a is included).
	writeFile(t, afs, filepath.Join(versionRoot, "lambda", "sub-b.md"), `---
title: 'SubB'
description: 'k6 sub-b command.'
weight: 10
---

# SubB
`)

	// Create included iota category.
	writeFile(t, afs, filepath.Join(versionRoot, "iota", "_index.md"), `---
title: 'Iota'
description: 'k6 iota.'
weight: 50
---

# Iota
`)

	// Create excluded categories.
	writeFile(t, afs, filepath.Join(versionRoot, "kappa", "_index.md"), `---
title: 'Kappa'
description: 'Getting started with k6.'
weight: 01
---

# Kappa
`)

	return afs, root
}

func writeFile(t *testing.T, afs fsext.Fs, path, content string) {
	t.Helper()
	if err := afs.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := fsext.WriteFile(afs, path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func loadOutputIndex(t *testing.T, afs fsext.Fs, outputDir, version string) (docs.Index, map[string]docs.Section) {
	t.Helper()

	data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "sections.json"))
	if err != nil {
		t.Fatalf("read sections.json: %v", err)
	}

	var idx docs.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("parse sections.json: %v", err)
	}

	if idx.Version != version {
		t.Errorf("Version = %q, want %q", idx.Version, version)
	}

	bySlug := make(map[string]docs.Section, len(idx.Sections))
	for _, s := range idx.Sections {
		bySlug[s.Slug] = s
	}

	return idx, bySlug
}

func assertNoExcludedCategories(t *testing.T, sections []docs.Section) {
	t.Helper()

	for _, s := range sections {
		if s.Category == "shared" {
			t.Errorf("section %q has excluded category %q", s.Slug, s.Category)
		}
	}
}

func TestRunWithMockDocs(t *testing.T) {
	t.Parallel()

	afs, docsPath := setupMockDocs(t)
	version := "v0.99.x"
	outputDir := "/output"

	if err := run(version, docsPath, outputDir, afs, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}

	_, bySlug := loadOutputIndex(t, afs, outputDir, version)

	// Check included sections exist.
	expectedSlugs := []string{
		"javascript-api",
		"javascript-api/k6-mod-a",
		"javascript-api/k6-mod-a/fn-one",
		"javascript-api/k6-mod-a/fn-two",
		"alpha",
		"alpha/topic-two",
		"alpha/topic-three",
		"lambda/sub-a",
		"lambda/sub-b",
		"iota",
		"kappa",
	}
	for _, slug := range expectedSlugs {
		if _, ok := bySlug[slug]; !ok {
			t.Errorf("expected section with slug %q, not found", slug)
		}
	}

	// Check excluded sections are absent.
	excludedSlugs := []string{
		"shared",
	}
	for _, slug := range excludedSlugs {
		if _, ok := bySlug[slug]; ok {
			t.Errorf("section %q should be excluded", slug)
		}
	}

	// Verify metadata.
	t.Run("javascript-api metadata", func(t *testing.T) {
		t.Parallel()

		requireSection(t, bySlug["javascript-api"], "JavaScript API", "javascript-api", true, 3)
	})

	t.Run("fn-one metadata", func(t *testing.T) {
		t.Parallel()

		requireSection(t, bySlug["javascript-api/k6-mod-a/fn-one"], "fnOne( url, [params] )", "javascript-api", false, 10)
	})

	// Verify children population.
	t.Run("k6-mod-a children", func(t *testing.T) {
		t.Parallel()

		// fn-one (weight 10) should come before fn-two (weight 20).
		requireChildren(t, bySlug["javascript-api/k6-mod-a"],
			"javascript-api/k6-mod-a/fn-one", "javascript-api/k6-mod-a/fn-two")
	})

	t.Run("javascript-api children", func(t *testing.T) {
		t.Parallel()

		requireChildren(t, bySlug["javascript-api"], "javascript-api/k6-mod-a")
	})

	t.Run("alpha children", func(t *testing.T) {
		t.Parallel()

		// topic-two (weight 400) before topic-three (weight 500).
		requireChildren(t, bySlug["alpha"], "alpha/topic-two", "alpha/topic-three")
	})

	t.Run("leaf node has empty children", func(t *testing.T) {
		t.Parallel()

		requireChildren(t, bySlug["alpha/topic-two"])
	})
}

func requireSection(t *testing.T, s docs.Section, title, category string, isIndex bool, weight int) {
	t.Helper()
	if s.Title != title {
		t.Errorf("Title = %q, want %q", s.Title, title)
	}
	if s.Category != category {
		t.Errorf("Category = %q, want %q", s.Category, category)
	}
	if s.IsIndex != isIndex {
		t.Errorf("IsIndex = %v, want %v", s.IsIndex, isIndex)
	}
	if s.Weight != weight {
		t.Errorf("Weight = %d, want %d", s.Weight, weight)
	}
}

func requireChildren(t *testing.T, s docs.Section, want ...string) {
	t.Helper()
	if s.Children == nil {
		t.Fatal("Children should be non-nil (empty slice, not nil)")
	}
	if len(s.Children) != len(want) {
		t.Fatalf("Children count = %d, want %d", len(s.Children), len(want))
	}
	for i, w := range want {
		if s.Children[i] != w {
			t.Errorf("Children[%d] = %q, want %q", i, s.Children[i], w)
		}
	}
}

func TestTransformedMarkdownContent(t *testing.T) {
	t.Parallel()

	afs, docsPath := setupMockDocs(t)
	version := "v0.99.x"
	outputDir := "/output-transformed"

	if err := run(version, docsPath, outputDir, afs, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}

	t.Run("shared content resolved", func(t *testing.T) {
		t.Parallel()

		data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "markdown", "javascript-api", "k6-mod-a", "_index.md"))
		if err != nil {
			t.Fatalf("read k6-mod-a _index.md: %v", err)
		}
		content := string(data)

		if strings.Contains(content, "docs/shared") {
			t.Error("shared shortcode should be resolved")
		}
		if !strings.Contains(content, "k6/mod-a module contains functionality") {
			t.Error("shared content should be inlined")
		}
	})

	t.Run("code tags preserved in bundle", func(t *testing.T) {
		t.Parallel()

		data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "markdown", "javascript-api", "k6-mod-a", "fn-one.md"))
		if err != nil {
			t.Fatalf("read fn-one.md: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "{{< code >}}") {
			t.Error("code shortcodes should be preserved in bundle (stripped at runtime)")
		}
		if !strings.Contains(content, "import modA from 'k6/mod-a'") {
			t.Error("code block content should be preserved")
		}
	})

	t.Run("admonition preserved in bundle", func(t *testing.T) {
		t.Parallel()

		data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "markdown", "alpha", "topic-two.md"))
		if err != nil {
			t.Fatalf("read topic-two.md: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "{{< admonition") {
			t.Error("admonition shortcode should be preserved in bundle (converted at runtime)")
		}
	})

	t.Run("frontmatter preserved in bundle", func(t *testing.T) {
		t.Parallel()

		data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "markdown", "alpha", "topic-three.md"))
		if err != nil {
			t.Fatalf("read topic-three.md: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "title: 'TopicThree'") {
			t.Error("frontmatter should be preserved in bundle (stripped at runtime)")
		}
		if !strings.Contains(content, "# TopicThree") {
			t.Error("markdown heading should be preserved")
		}
	})

	t.Run("version placeholder preserved in bundle", func(t *testing.T) {
		t.Parallel()

		data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "markdown", "javascript-api", "k6-mod-a", "fn-one.md"))
		if err != nil {
			t.Fatalf("read fn-one.md: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "<K6_VERSION>") {
			t.Error("version placeholder should be preserved in bundle (replaced at runtime)")
		}
	})
}

func TestBestPracticesWritten(t *testing.T) {
	t.Parallel()

	afs, docsPath := setupMockDocs(t)
	version := "v0.99.x"
	outputDir := "/output-bestpractices"

	if err := run(version, docsPath, outputDir, afs, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "best_practices.md"))
	if err != nil {
		t.Fatalf("read best_practices.md: %v", err)
	}

	content := string(data)
	requiredTopics := []string{
		"Test Structure",
		"Performance",
		"Error Handling",
		"Data Management",
		"Authentication",
		"Monitoring",
		"Scenarios",
		"Modules",
		"Browser",
	}
	for _, topic := range requiredTopics {
		if !strings.Contains(content, topic) {
			t.Errorf("best_practices.md should contain %q", topic)
		}
	}

	// Should contain code examples.
	if !strings.Contains(content, "```javascript") {
		t.Error("best_practices.md should contain JavaScript code examples")
	}
}

func TestSlugDerivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		relPath string
		want    string
	}{
		{"javascript-api/k6-mod-a/_index.md", "javascript-api/k6-mod-a"},
		{"javascript-api/k6-mod-a/fn-one.md", "javascript-api/k6-mod-a/fn-one"},
		{"alpha/topic-one/_index.md", "alpha/topic-one"},
		{"alpha/topic-two.md", "alpha/topic-two"},
		{"beta/_index.md", "beta"},
		{"lambda/sub-a.md", "lambda/sub-a"},
	}

	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			t.Parallel()
			got := slugFromRelPath(tt.relPath)
			if got != tt.want {
				t.Errorf("slugFromRelPath(%q) = %q, want %q", tt.relPath, got, tt.want)
			}
		})
	}
}

func TestCategoryDerivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		slug string
		want string
	}{
		{"javascript-api/k6-mod-a/fn-one", "javascript-api"},
		{"alpha/topic-two", "alpha"},
		{"beta", "beta"},
		{"lambda/sub-a", "lambda"},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			t.Parallel()
			got := categoryFromSlug(tt.slug)
			if got != tt.want {
				t.Errorf("categoryFromSlug(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    frontmatter
	}{
		{
			name:    "typical frontmatter",
			content: "---\ntitle: 'TopicTwo'\ndescription: 'Validate conditions.'\nweight: 400\n---\n\n# TopicTwo",
			want:    frontmatter{Title: "TopicTwo", Description: "Validate conditions.", Weight: 400},
		},
		{
			name:    "no frontmatter",
			content: "# Just markdown",
			want:    frontmatter{},
		},
		{
			name:    "frontmatter with extra fields",
			content: "---\ntitle: 'Test'\naliases:\n  - /old/path\nweight: 5\n---\ncontent",
			want:    frontmatter{Title: "Test", Weight: 5},
		},
		{
			name:    "empty content",
			content: "",
			want:    frontmatter{},
		},
		{
			name:    "duplicate keys keeps first",
			content: "---\ntitle: 'First'\ndescription: 'First desc'\ndescription: 'Second desc'\nweight: 10\n---\n\n# Body",
			want:    frontmatter{Title: "First", Description: "First desc", Weight: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseFrontmatter(tt.content)
			if err != nil {
				t.Fatalf("parseFrontmatter: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDeduplicateYAMLKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no duplicates",
			input: "title: 'Hello'\ndescription: 'World'\nweight: 1",
			want:  "title: 'Hello'\ndescription: 'World'\nweight: 1",
		},
		{
			name:  "duplicate description",
			input: "title: 'Hello'\ndescription: 'First'\ndescription: 'Second'\nweight: 1",
			want:  "title: 'Hello'\ndescription: 'First'\nweight: 1",
		},
		{
			name:  "preserves indented lines",
			input: "title: 'Hello'\naliases:\n  - /old\n  - /older\ntitle: 'Duplicate'",
			want:  "title: 'Hello'\naliases:\n  - /old\n  - /older",
		},
		{
			name:  "preserves comments",
			input: "# comment\ntitle: 'Hello'\ntitle: 'Dup'",
			want:  "# comment\ntitle: 'Hello'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := deduplicateYAMLKeys(tt.input)
			if got != tt.want {
				t.Errorf("deduplicateYAMLKeys():\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestPopulateChildren(t *testing.T) {
	t.Parallel()

	sections := []docs.Section{
		{Slug: "alpha", IsIndex: true, Weight: 1},
		{Slug: "alpha/topic-two", Weight: 400},
		{Slug: "alpha/topic-three", Weight: 200},
		{Slug: "alpha/topic-one", IsIndex: true, Weight: 300},
		{Slug: "alpha/topic-one/executors", IsIndex: true, Weight: 1},
		{Slug: "alpha/topic-one/executors/shared-iterations", Weight: 1},
	}

	populateChildren(sections)

	// alpha should have topic-two, topic-three, topic-one as direct children.
	// Sorted by weight: topic-three (200), topic-one (300), topic-two (400).
	parent := sections[0]
	if len(parent.Children) != 3 {
		t.Fatalf("alpha children: got %d, want 3", len(parent.Children))
	}
	if parent.Children[0] != "alpha/topic-three" {
		t.Errorf("Children[0] = %q, want %q", parent.Children[0], "alpha/topic-three")
	}
	if parent.Children[1] != "alpha/topic-one" {
		t.Errorf("Children[1] = %q, want %q", parent.Children[1], "alpha/topic-one")
	}
	if parent.Children[2] != "alpha/topic-two" {
		t.Errorf("Children[2] = %q, want %q", parent.Children[2], "alpha/topic-two")
	}

	// alpha/topic-one should have executors as only direct child.
	topicOne := sections[3]
	if len(topicOne.Children) != 1 {
		t.Fatalf("topic-one children: got %d, want 1", len(topicOne.Children))
	}
	if topicOne.Children[0] != "alpha/topic-one/executors" {
		t.Errorf("Children[0] = %q, want %q", topicOne.Children[0], "alpha/topic-one/executors")
	}

	// alpha/topic-one/executors should have shared-iterations.
	executors := sections[4]
	if len(executors.Children) != 1 {
		t.Fatalf("executors children: got %d, want 1", len(executors.Children))
	}

	// Non-index leaf nodes should have empty (non-nil) children.
	topicTwo := sections[1]
	if topicTwo.Children == nil {
		t.Error("leaf Children should be non-nil")
	}
	if len(topicTwo.Children) != 0 {
		t.Errorf("leaf Children count = %d, want 0", len(topicTwo.Children))
	}
}

func TestSlugCollisionPrefersIndex(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	version := "v0.99.x"
	root := "/collision-test"
	versionRoot := filepath.Join(root, "docs", "sources", "k6", version)

	// Create a regular file and an _index.md that produce the same slug.
	// javascript-api/k6-mod-a/child-a.md  -> slug: javascript-api/k6-mod-a/child-a
	// javascript-api/k6-mod-a/child-a/_index.md -> slug: javascript-api/k6-mod-a/child-a
	writeFile(t, afs, filepath.Join(versionRoot, "_index.md"), "---\ntitle: root\n---\n")
	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "_index.md"), "---\ntitle: 'JS API'\nweight: 1\n---\n")
	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "k6-mod-a", "_index.md"), "---\ntitle: 'k6/mod-a'\nweight: 1\n---\n")
	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "k6-mod-a", "child-a.md"),
		"---\ntitle: 'child-a function'\nweight: 10\n---\n\nA function.\n")
	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "k6-mod-a", "child-a", "_index.md"),
		"---\ntitle: 'ChildA class'\nweight: 20\n---\n\nA class with children.\n")
	writeFile(t, afs, filepath.Join(versionRoot, "javascript-api", "k6-mod-a", "child-a", "set.md"),
		"---\ntitle: 'set'\nweight: 1\n---\n\nSet an item.\n")

	outputDir := "/output-collision"
	if err := run(version, root, outputDir, afs, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "sections.json"))
	if err != nil {
		t.Fatalf("read sections.json: %v", err)
	}

	var idx docs.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("parse sections.json: %v", err)
	}

	// Count how many sections have the colliding slug.
	slug := "javascript-api/k6-mod-a/child-a"
	var matches []docs.Section
	for _, s := range idx.Sections {
		if s.Slug == slug {
			matches = append(matches, s)
		}
	}

	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 section with slug %q, got %d", slug, len(matches))
	}

	// The _index.md version should win (it has children).
	if !matches[0].IsIndex {
		t.Errorf("expected the _index.md version to win the slug collision, got IsIndex=false")
	}
	if matches[0].Title != "ChildA class" {
		t.Errorf("Title = %q, want %q", matches[0].Title, "ChildA class")
	}
}

func TestRunWithRealDocs(t *testing.T) {
	t.Parallel()

	afs := fsext.NewOsFs()

	k6DocsPath, _ := syscall.Getenv("K6_DOCS_PATH")
	if k6DocsPath == "" {
		home, _ := syscall.Getenv("HOME")
		k6DocsPath = filepath.Join(home, "grafana", "k6-docs")
	}
	if _, err := afs.Stat(k6DocsPath); err != nil {
		t.Skipf("skipping integration test: k6-docs not found at %s", k6DocsPath)
	}

	outputDir := filepath.Join(t.TempDir(), "real-output")
	version := "v1.5.x"

	if err := run(version, k6DocsPath, outputDir, afs, io.Discard); err != nil {
		t.Fatalf("run with real docs: %v", err)
	}

	idx, bySlug := loadOutputIndex(t, afs, outputDir, version)

	// Should have a reasonable number of sections.
	if len(idx.Sections) < 50 {
		t.Errorf("expected at least 50 sections, got %d", len(idx.Sections))
	}

	// Spot-check some expected sections.
	spotChecks := []string{
		"javascript-api",
		"javascript-api/k6-http",
		"javascript-api/k6-http/get",
		"using-k6",
		"using-k6/checks",
		"reference/glossary",
	}
	for _, slug := range spotChecks {
		if _, ok := bySlug[slug]; !ok {
			t.Errorf("expected section %q in real output", slug)
		}
	}

	assertNoExcludedCategories(t, idx.Sections)

	// Check that a prepared markdown file exists. Bundle content is raw-ish:
	// shared shortcodes resolved but frontmatter/other shortcodes preserved
	// (they are stripped at runtime).
	checksPath := filepath.Join(outputDir, "markdown", "using-k6", "checks.md")
	checksData, err := fsext.ReadFile(afs, checksPath)
	if err != nil {
		t.Fatalf("read prepared checks.md: %v", err)
	}
	if !strings.HasPrefix(string(checksData), "---") {
		t.Error("prepared markdown should still have frontmatter (stripped at runtime)")
	}
	// Shared shortcodes should be resolved, but other shortcodes are preserved.
	if strings.Contains(string(checksData), "docs/shared") {
		t.Error("shared shortcodes should be resolved at prepare time")
	}

	// best_practices.md should exist.
	if _, err := afs.Stat(filepath.Join(outputDir, "best_practices.md")); err != nil {
		t.Error("best_practices.md should exist in output")
	}
}

func TestRunWithExactVersionNoVPrefix(t *testing.T) {
	t.Parallel()

	// The docs directory uses the wildcard form v0.99.x. When the caller
	// passes an exact version without the "v" prefix (e.g. "0.99.3"),
	// MapToWildcard must still produce "v0.99.x" to match the directory.
	afs, docsPath := setupMockDocs(t)
	outputDir := "/output-novprefix"

	if err := run("0.99.3", docsPath, outputDir, afs, io.Discard); err != nil {
		t.Fatalf("run with bare version (no v prefix): %v", err)
	}

	data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "sections.json"))
	if err != nil {
		t.Fatalf("read sections.json: %v", err)
	}

	var idx docs.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("parse sections.json: %v", err)
	}

	if idx.Version != "0.99.3" {
		t.Errorf("Version = %q, want %q (original version should be preserved)", idx.Version, "0.99.3")
	}

	if len(idx.Sections) == 0 {
		t.Error("expected sections to be populated")
	}
}

func TestMissingVersion(t *testing.T) {
	t.Parallel()

	afs, docsPath := setupMockDocs(t)
	outputDir := "/output-missing"

	err := run("v999.999.x", docsPath, outputDir, afs, io.Discard)
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
	if !strings.Contains(err.Error(), "version root not found") {
		t.Errorf("error = %q, expected to mention 'version root not found'", err.Error())
	}
}

func TestRunWithExactVersion(t *testing.T) {
	t.Parallel()

	// The docs directory uses the wildcard form v0.99.x, but the caller
	// passes an exact version like v0.99.3. The run function must map the
	// exact version to the wildcard directory automatically.
	afs, docsPath := setupMockDocs(t)
	outputDir := "/output-exact"

	if err := run("v0.99.3", docsPath, outputDir, afs, io.Discard); err != nil {
		t.Fatalf("run with exact version: %v", err)
	}

	// The index should preserve the original exact version.
	data, err := fsext.ReadFile(afs, filepath.Join(outputDir, "sections.json"))
	if err != nil {
		t.Fatalf("read sections.json: %v", err)
	}

	var idx docs.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("parse sections.json: %v", err)
	}

	if idx.Version != "v0.99.3" {
		t.Errorf("Version = %q, want %q (exact version should be preserved)", idx.Version, "v0.99.3")
	}

	if len(idx.Sections) == 0 {
		t.Error("expected sections to be populated")
	}
}

func TestEnsureDocsRepoAutoClone(t *testing.T) {
	t.Parallel()

	// Create a local bare git repo to clone from instead of hitting GitHub.
	localRepo := t.TempDir()
	setupLocalGitRepo(t, localRepo)

	afs := fsext.NewOsFs()
	var stderr bytes.Buffer

	path, cleanup, err := ensureDocsRepo("", localRepo, afs, &stderr)
	if err != nil {
		t.Fatalf("ensureDocsRepo: %v", err)
	}

	if path == "" {
		t.Fatal("ensureDocsRepo returned empty path")
	}

	// Verify the cloned directory has the expected structure.
	docsDir := filepath.Join(path, "docs")
	if _, statErr := os.Stat(docsDir); statErr != nil {
		t.Errorf("cloned repo missing docs/ directory: %v", statErr)
	}

	if !strings.Contains(stderr.String(), "Cloning") {
		t.Error("expected 'Cloning' in stderr output")
	}

	// Cleanup should remove the directory.
	if cleanup != nil {
		cleanup()
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("cleanup should have removed the cloned directory")
	}
}

// setupLocalGitRepo creates a minimal git repo with a docs/ directory
// so ensureDocsRepo can clone it locally instead of hitting GitHub.
func setupLocalGitRepo(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", ".gitkeep"), nil, 0o644); err != nil {
		t.Fatalf("write .gitkeep: %v", err)
	}

	for _, args := range [][]string{
		{"git", "init"},
		{"git", "add", "."},
		{"git", "-c", "user.name=test", "-c", "user.email=test@test", "commit", "-m", "init"},
	} {
		cmd := exec.CommandContext(t.Context(), args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
}
