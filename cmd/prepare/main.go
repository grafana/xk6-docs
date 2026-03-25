// Command prepare processes the k6-docs repository into a doc bundle
// suitable for embedding. It walks the documentation tree, transforms
// Hugo shortcodes into clean markdown, and produces:
//   - markdown/ — transformed .md files
//   - sections.json — structured index of all sections
//   - best_practices.md — a comprehensive best practices guide
package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	docs "github.com/grafana/xk6-docs"
	"go.k6.io/k6/lib/fsext"
	"gopkg.in/yaml.v3"
)

// frontmatter holds the YAML fields we extract from each doc file.
type frontmatter struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Weight      int    `yaml:"weight"`
}

func main() {
	log.SetFlags(0)

	var (
		k6Version  string
		k6DocsPath string
		outputDir  string
	)

	flag.StringVar(&k6Version, "k6-version", "", "k6 docs version (e.g. v1.5.x) — required")
	flag.StringVar(&k6DocsPath, "k6-docs-path", "", "local path to k6-docs repo (cloned if empty)")
	flag.StringVar(&outputDir, "output-dir", "dist/", "output directory")
	flag.Parse()

	if k6Version == "" {
		log.Fatal("--k6-version is required")
	}

	afs := fsext.NewOsFs()
	if err := run(k6Version, k6DocsPath, outputDir, afs, log.Writer()); err != nil {
		log.Fatal(err)
	}
}

func run(
	k6Version, k6DocsPath, outputDir string,
	afs fsext.Fs, stderr io.Writer,
) error {
	// Step 1: ensure we have the k6-docs repo.
	docsPath, cleanup, err := ensureDocsRepo(k6DocsPath, defaultRepoURL, afs, stderr)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// The k6-docs repo uses wildcard directories (e.g. "v1.6.x"), so convert
	// exact versions like "v1.6.1" to the wildcard form for the path lookup.
	docsVersion := docs.MapToWildcard(k6Version)
	versionRoot := filepath.Join(docsPath, "docs", "sources", "k6", docsVersion)
	if _, err := afs.Stat(filepath.Clean(versionRoot)); err != nil {
		return fmt.Errorf("version root not found: %w", err)
	}

	// Step 2: build shared content map.
	sharedDir := filepath.Join(versionRoot, "shared")
	sharedContent, err := buildSharedContentMap(afs, sharedDir)
	if err != nil {
		return fmt.Errorf("build shared content: %w", err)
	}

	// Step 3: walk documentation files and collect sections.
	markdownDir := filepath.Join(outputDir, "markdown")
	sharedRel, _ := filepath.Rel(versionRoot, sharedDir)
	sections, err := walkAndProcess(afs, versionRoot, markdownDir, sharedContent, filepath.ToSlash(sharedRel))
	if err != nil {
		return fmt.Errorf("walk docs: %w", err)
	}

	// Step 4: populate children.
	populateChildren(sections)

	// Step 5: write sections.json.
	idx := docs.Index{
		Version:  k6Version,
		Sections: sections,
	}
	if err := writeSectionsJSON(afs, outputDir, idx); err != nil {
		return err
	}

	// Step 6: write best_practices.md.
	if err := writeBestPractices(afs, outputDir); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(stderr, "Done: sections written")
	return nil
}

const defaultRepoURL = "https://github.com/grafana/k6-docs.git"

// ensureDocsRepo returns the path to the k6-docs repo. If k6DocsPath is empty,
// it clones from repoURL into a temp directory and returns a cleanup function.
func ensureDocsRepo(
	k6DocsPath, repoURL string, afs fsext.Fs, stderr io.Writer,
) (string, func(), error) {
	if k6DocsPath != "" {
		return k6DocsPath, nil, nil
	}

	tmpDir, err := mkTempDir(afs)
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	_, _ = fmt.Fprintln(stderr, "Cloning k6-docs repository...")
	_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:      repoURL,
		Depth:    1,
		Progress: stderr,
	})
	if err != nil {
		_ = afs.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("clone k6-docs: %w", err)
	}

	cleanup := func() { _ = afs.RemoveAll(tmpDir) }
	return tmpDir, cleanup, nil
}

func mkTempDir(afs fsext.Fs) (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	dir := filepath.Join("/tmp", fmt.Sprintf("k6-docs-%x", buf))
	if err := afs.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}

// buildSharedContentMap reads all .md files under the shared directory and
// returns a map keyed by the relative path (e.g. "javascript-api/module.md").
func buildSharedContentMap(afs fsext.Fs, sharedDir string) (map[string]string, error) {
	m := make(map[string]string)

	info, err := afs.Stat(filepath.Clean(sharedDir))
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return m, nil
	}
	if err != nil {
		return m, err
	}

	err = fsext.Walk(afs, sharedDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, err := filepath.Rel(sharedDir, path)
		if err != nil {
			return err
		}
		data, err := fsext.ReadFile(afs, filepath.Clean(path))
		if err != nil {
			return fmt.Errorf("read shared %s: %w", rel, err)
		}
		m[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	return m, err
}

// parseFrontmatter extracts YAML frontmatter from content.
func parseFrontmatter(content string) (frontmatter, error) {
	var fm frontmatter
	yamlBlock, _, ok := docs.SplitFrontmatter(content)
	if !ok {
		return fm, nil
	}
	yamlBlock = deduplicateYAMLKeys(yamlBlock)
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return fm, fmt.Errorf("parse yaml: %w", err)
	}
	return fm, nil
}

// deduplicateYAMLKeys removes duplicate top-level YAML keys, keeping only
// the first occurrence of each key. This handles the ~60 k6-docs files that
// have duplicate "description:" keys, which cause yaml.v3 to error.
func deduplicateYAMLKeys(yamlBlock string) string {
	seen := make(map[string]bool)
	var lines []string
	for line := range strings.SplitSeq(yamlBlock, "\n") {
		if idx := strings.Index(line, ":"); idx > 0 && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
			key := strings.TrimSpace(line[:idx])
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// slugFromRelPath derives the slug from a relative path.
// Rules: strip .md, if _index.md use parent dir, path uses forward slashes.
func slugFromRelPath(relPath string) string {
	relPath = filepath.ToSlash(relPath)
	base := filepath.Base(relPath)
	if base == "_index.md" {
		return filepath.ToSlash(filepath.Dir(relPath))
	}
	return strings.TrimSuffix(relPath, ".md")
}

// categoryFromSlug extracts the first path segment as the category.
func categoryFromSlug(slug string) string {
	if before, _, found := strings.Cut(slug, "/"); found {
		return before
	}
	return slug
}

// walkAndProcess walks the version root, processes included .md files,
// and returns the collected sections.
func walkAndProcess(
	afs fsext.Fs, versionRoot, markdownDir string, sharedContent map[string]string, skipDir string,
) ([]docs.Section, error) {
	// Use a map to deduplicate sections by slug. When a slug collision
	// occurs (e.g. child.md and child/_index.md both produce
	// "javascript-api/k6-module/child"), prefer the _index.md entry
	// because it represents a section with children.
	sectionMap := make(map[string]docs.Section)
	var slugOrder []string

	err := fsext.Walk(afs, versionRoot, func(path string, info fs.FileInfo, err error) error {
		return processEntry(afs, path, info, err, versionRoot, markdownDir, sharedContent, skipDir, sectionMap, &slugOrder)
	})

	// Rebuild the slice in walk order.
	sections := make([]docs.Section, 0, len(slugOrder))
	for _, slug := range slugOrder {
		sections = append(sections, sectionMap[slug])
	}

	return sections, err
}

func processEntry(
	afs fsext.Fs,
	path string, info fs.FileInfo, err error,
	versionRoot, markdownDir string,
	sharedContent map[string]string,
	skipDir string,
	sectionMap map[string]docs.Section,
	slugOrder *[]string,
) error {
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(versionRoot, path)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)

	if info.IsDir() {
		if rel == skipDir {
			return filepath.SkipDir
		}
		return nil
	}

	if !strings.HasSuffix(rel, ".md") {
		return nil
	}

	// Skip the version root _index.md.
	if rel == "_index.md" {
		return nil
	}

	content, err := fsext.ReadFile(afs, filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}

	fm, err := parseFrontmatter(string(content))
	if err != nil {
		log.Printf("warning: %s: %v", rel, err)
	}

	transformed := docs.PrepareTransform(string(content), sharedContent)

	slug := slugFromRelPath(rel)
	category := categoryFromSlug(slug)
	isIndex := filepath.Base(path) == "_index.md"

	// Write transformed markdown.
	outPath := filepath.Join(markdownDir, rel)
	if err := afs.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
	}
	if err := fsext.WriteFile(afs, outPath, []byte(transformed), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	sec := docs.Section{
		Slug:        slug,
		RelPath:     rel,
		Title:       fm.Title,
		Description: fm.Description,
		Weight:      fm.Weight,
		Category:    category,
		IsIndex:     isIndex,
	}

	// Handle slug collisions: prefer _index.md over plain .md files.
	if existing, ok := sectionMap[slug]; ok {
		if isIndex && !existing.IsIndex {
			sectionMap[slug] = sec
		}
	} else {
		*slugOrder = append(*slugOrder, slug)
		sectionMap[slug] = sec
	}

	return nil
}

// populateChildren sets the Children field for each _index section.
// A child is a section whose slug starts with parent slug + "/" and has
// no further "/" after that prefix (direct child only).
func populateChildren(sections []docs.Section) {
	for i := range sections {
		if !sections[i].IsIndex {
			continue
		}

		parentSlug := sections[i].Slug
		prefix := parentSlug + "/"

		// Collect direct children.
		type child struct {
			slug   string
			weight int
		}
		var children []child

		for j := range sections {
			if i == j {
				continue
			}
			s := sections[j].Slug
			if !strings.HasPrefix(s, prefix) {
				continue
			}
			remainder := s[len(prefix):]
			if strings.Contains(remainder, "/") {
				continue
			}
			children = append(children, child{slug: s, weight: sections[j].Weight})
		}

		sort.Slice(children, func(a, b int) bool {
			return children[a].weight < children[b].weight
		})

		slugs := make([]string, len(children))
		for k, c := range children {
			slugs[k] = c.slug
		}
		sections[i].Children = slugs
	}

	// Ensure non-index sections have empty (non-nil) Children.
	for i := range sections {
		if sections[i].Children == nil {
			sections[i].Children = []string{}
		}
	}
}

// writeSectionsJSON writes the index to sections.json in the output directory.
func writeSectionsJSON(afs fsext.Fs, outputDir string, idx docs.Index) error {
	if err := afs.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sections: %w", err)
	}

	outPath := filepath.Join(outputDir, "sections.json")
	if err := fsext.WriteFile(afs, outPath, data, 0o600); err != nil {
		return fmt.Errorf("write sections.json: %w", err)
	}

	log.Printf("Wrote %s", outPath)
	return nil
}

// writeBestPractices writes a comprehensive best practices guide.
func writeBestPractices(afs fsext.Fs, outputDir string) error {
	if err := afs.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(outputDir, "best_practices.md")
	if err := fsext.WriteFile(afs, outPath, []byte(bestPracticesContent), 0o600); err != nil {
		return fmt.Errorf("write best_practices.md: %w", err)
	}

	log.Printf("Wrote %s", outPath)
	return nil
}

//go:embed best_practices.md
var bestPracticesContent string
