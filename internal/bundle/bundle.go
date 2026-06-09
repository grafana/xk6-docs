// Package bundle transforms a k6-docs working tree into a doc bundle
// (sections.json + a markdown/ subtree). It is the shared pipeline used both
// by the standalone cmd/prepare tool and by the k6 x docs --source preview.
package bundle

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	docs "github.com/grafana/xk6-docs/docs"
	"gopkg.in/yaml.v3"
)

// frontmatter holds the YAML fields we extract from each doc file.
type frontmatter struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Weight      int      `yaml:"weight"`
	Aliases     []string `yaml:"aliases"`
}

// Build transforms the k6-docs working tree at k6DocsPath into a bundle
// (sections.json + markdown/) under outputDir, for the given version. The
// version may be exact ("v1.6.1") or wildcard ("v1.6.x", "next"); it is mapped
// to the wildcard directory form for the source lookup.
func Build(k6Version, k6DocsPath, outputDir string, afs docs.FS, _ io.Writer) error {
	// The k6-docs repo uses wildcard directories (e.g. "v1.6.x"), so convert
	// exact versions like "v1.6.1" to the wildcard form for the path lookup.
	docsVersion := docs.VersionWildcard(k6Version)
	versionRoot := filepath.Join(k6DocsPath, "docs", "sources", "k6", docsVersion)
	if _, err := afs.Stat(filepath.Clean(versionRoot)); err != nil {
		return fmt.Errorf("version root not found: %w", err)
	}

	sharedDir := filepath.Join(versionRoot, "shared")
	sharedContent, err := buildSharedContentMap(afs, sharedDir)
	if err != nil {
		return fmt.Errorf("build shared content: %w", err)
	}

	markdownDir := filepath.Join(outputDir, "markdown")
	sharedRel, _ := filepath.Rel(versionRoot, sharedDir)
	sections, err := walkAndProcess(afs, versionRoot, markdownDir, sharedContent, filepath.ToSlash(sharedRel))
	if err != nil {
		return fmt.Errorf("walk docs: %w", err)
	}

	populateChildren(sections)

	idx := &docs.Index{Version: k6Version, Sections: sections}
	if err := writeSectionsJSON(afs, outputDir, idx); err != nil {
		return err
	}

	return writeBestPractices(afs, outputDir)
}

// buildSharedContentMap reads all .md files under the shared directory and
// returns a map keyed by the relative path (e.g. "javascript-api/module.md").
func buildSharedContentMap(afs docs.FS, sharedDir string) (map[string]string, error) {
	m := make(map[string]string)

	info, err := afs.Stat(filepath.Clean(sharedDir))
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return m, nil
	}
	if err != nil {
		return m, err
	}

	err = filepath.WalkDir(sharedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, err := filepath.Rel(sharedDir, path)
		if err != nil {
			return err
		}
		data, err := afs.ReadFile(filepath.Clean(path))
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
	afs docs.FS, versionRoot, markdownDir string, sharedContent map[string]string, skipDir string,
) ([]docs.Section, error) {
	// Use a map to deduplicate sections by slug. When a slug collision
	// occurs (e.g. child.md and child/_index.md both produce
	// "javascript-api/k6-module/child"), prefer the _index.md entry
	// because it represents a section with children.
	sectionMap := make(map[string]docs.Section)
	var slugOrder []string

	err := filepath.WalkDir(versionRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		return processEntry(afs, path, info, versionRoot, markdownDir, sharedContent, skipDir, sectionMap, &slugOrder)
	})

	// Rebuild the slice in walk order.
	sections := make([]docs.Section, 0, len(slugOrder))
	for _, slug := range slugOrder {
		sections = append(sections, sectionMap[slug])
	}

	return sections, err
}

func processEntry(
	afs docs.FS,
	path string, info fs.FileInfo,
	versionRoot, markdownDir string,
	sharedContent map[string]string,
	skipDir string,
	sectionMap map[string]docs.Section,
	slugOrder *[]string,
) error {
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

	content, err := afs.ReadFile(filepath.Clean(path))
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
	if err := afs.WriteFile(outPath, []byte(transformed), 0o600); err != nil {
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
		Aliases:     resolveAliases(slug, fm.Aliases),
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

// resolveAliases converts relative Hugo aliases into absolute slugs.
// Hugo aliases are relative to the page's parent directory, e.g. a page at
// "alpha/topic-two" with alias "../legacy/checks" resolves to "legacy/checks".
func resolveAliases(slug string, raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	parentDir := filepath.ToSlash(filepath.Dir(slug))
	resolved := make([]string, 0, len(raw))
	for _, a := range raw {
		a = strings.TrimSuffix(strings.TrimSpace(a), "/")
		if a == "" {
			continue
		}
		joined := filepath.Join(parentDir, a)
		cleaned := filepath.ToSlash(filepath.Clean(joined))
		// Strip leading "../" — aliases can't escape the doc root.
		for strings.HasPrefix(cleaned, "../") {
			cleaned = cleaned[3:]
		}
		resolved = append(resolved, cleaned)
	}
	return resolved
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
func writeSectionsJSON(afs docs.FS, outputDir string, idx *docs.Index) error {
	if err := afs.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sections: %w", err)
	}

	outPath := filepath.Join(outputDir, "sections.json")
	if err := afs.WriteFile(outPath, data, 0o600); err != nil {
		return fmt.Errorf("write sections.json: %w", err)
	}

	return nil
}

// writeBestPractices writes a comprehensive best practices guide.
func writeBestPractices(afs docs.FS, outputDir string) error {
	markdownDir := filepath.Join(outputDir, "markdown")
	if err := afs.MkdirAll(markdownDir, 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(markdownDir, "best_practices.md")
	if err := afs.WriteFile(outPath, []byte(bestPracticesContent), 0o600); err != nil {
		return fmt.Errorf("write best_practices.md: %w", err)
	}

	return nil
}

//go:embed best_practices.md
var bestPracticesContent string

// NewOSFS returns a docs.FS backed by the operating system.
func NewOSFS() docs.FS { return osFS{} }

// osFS is the filesystem boundary. All os calls are confined here.
type osFS struct{}

func (osFS) Open(name string) (fs.File, error) {
	return os.Open(name) //nolint:forbidigo,gosec // FS boundary
}

func (osFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name) //nolint:forbidigo,gosec // FS boundary
}

func (osFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name) //nolint:forbidigo // FS boundary
}

func (osFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name) //nolint:forbidigo // FS boundary
}

func (osFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm) //nolint:forbidigo // FS boundary
}

func (osFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm) //nolint:forbidigo // FS boundary
}

func (osFS) RemoveAll(path string) error {
	return os.RemoveAll(path) //nolint:forbidigo // FS boundary
}
