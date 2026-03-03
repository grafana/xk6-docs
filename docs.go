package docs

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"go.k6.io/k6/lib/fsext"
)

// childName returns the short name of a child relative to its parent.
// If the child slug starts with parentSlug+"/", the prefix is stripped.
// Then, if the remaining name starts with the parent's last segment + "-",
// that redundant prefix is also stripped (e.g. cookiejar-clear → clear).
func childName(childSlug, parentSlug string) string {
	if strings.HasPrefix(childSlug, parentSlug+"/") {
		name := childSlug[len(parentSlug)+1:]
		var parentName string
		if i := strings.LastIndex(parentSlug, "/"); i >= 0 {
			parentName = parentSlug[i+1:]
		} else {
			parentName = parentSlug
		}
		return strings.TrimPrefix(name, parentName+"-")
	}
	if i := strings.LastIndex(childSlug, "/"); i >= 0 {
		return childSlug[i+1:]
	}
	return childSlug
}

// slugToArgs converts a documentation slug to CLI args for display.
// For javascript-api slugs, strips the prefix and k6- from the first segment.
func slugToArgs(slug string) string {
	parts := strings.Split(slug, "/")
	if parts[0] == "javascript-api" && len(parts) > 1 {
		parts = parts[1:]
		parts[0] = strings.TrimPrefix(parts[0], "k6-")
	}
	return strings.Join(parts, " ")
}

// printExample prints a blockquote example hint preceded by a blank line.
func printExample(w io.Writer, example string) {
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "> Example: `%s`\n", example)
}

// printSubtopics prints a subtopics block: bold header, bullet list, and example hint.
func printSubtopics(w io.Writer, path string, names []string) {
	_, _ = fmt.Fprintf(w, "**%s subtopics:**\n", path)
	for _, name := range names {
		_, _ = fmt.Fprintf(w, "- %s\n", name)
	}
	printExample(w, fmt.Sprintf("k6 x docs %s/<subtopic>", path))
}

// printTOC prints the table of contents as a flat slug list.
func printTOC(w io.Writer, idx *Index, version string) {
	_, _ = fmt.Fprintf(w, "# k6 %s\n", version)

	for _, cat := range idx.TopLevel() {
		_, _ = fmt.Fprintf(w, "- %s\n", cat.Slug)
	}

	printExample(w, "k6 x docs <topic>")
}

// printSection prints a section's markdown content, read from the cache dir.
// If the section has children, a subtopics footer is appended.
func printSection(afs fsext.Fs, w io.Writer, idx *Index, section *Section, cacheDir, version string) {
	content := strings.TrimSpace(readAndTransform(afs, cacheDir, section.RelPath, version))
	if content != "" {
		_, _ = fmt.Fprintln(w, content)
	}

	children := idx.Children(section.Slug)
	if len(children) > 0 {
		path := strings.ReplaceAll(slugToArgs(section.Slug), " ", "/")

		if content != "" {
			_, _ = fmt.Fprintln(w)
			_, _ = fmt.Fprintln(w, "---")
		}

		names := make([]string, 0, len(children))
		for _, c := range children {
			names = append(names, childName(c.Slug, section.Slug))
		}
		printSubtopics(w, path, names)
	}
}

// searchGroupKey returns the grouping key for a search result.
// JavaScript API sections group by module (second segment); others by first segment.
func searchGroupKey(slug string) string {
	parts := strings.SplitN(slug, "/", 3)
	if parts[0] == "javascript-api" && len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}

// printSearch prints search results as an indented tree, no descriptions.
func printSearch(afs fsext.Fs, w io.Writer, idx *Index, term, cacheDir, version string) {
	readContent := func(slug string) string {
		sec, ok := idx.Lookup(slug)
		if !ok {
			return ""
		}
		return readAndTransform(afs, cacheDir, sec.RelPath, version)
	}

	results := idx.Search(term, readContent)

	if len(results) == 0 {
		_, _ = fmt.Fprintln(w, "(no results)")
		return
	}

	// Build a set of matched slugs and group results by parent topic.
	matched := make(map[string]*Section, len(results))
	groups := make(map[string][]*Section)
	var groupOrder []string

	for _, sec := range results {
		matched[sec.Slug] = sec

		// Skip bare "javascript-api" — it's the TOC default.
		if sec.Slug == "javascript-api" {
			continue
		}

		key := searchGroupKey(sec.Slug)
		if _, exists := groups[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], sec)
	}

	// Sort groups alphabetically.
	sort.Strings(groupOrder)

	var firstChildSlug string

	for _, key := range groupOrder {
		members := groups[key]

		// Sort items within group alphabetically by slug.
		sort.Slice(members, func(i, j int) bool {
			return members[i].Slug < members[j].Slug
		})

		// Check if the group topic itself is a matched result.
		// For JS API modules, the group slug is "javascript-api/{key}".
		// For others, it's just "{key}".
		groupSlug := key
		if _, ok := idx.Lookup("javascript-api/" + key); ok {
			if members[0].Slug == "javascript-api/"+key || strings.HasPrefix(members[0].Slug, "javascript-api/"+key+"/") {
				groupSlug = "javascript-api/" + key
			}
		}

		_, _ = fmt.Fprintf(w, "- %s\n", key)

		// Collect children (items that aren't the group header itself).
		// Deduplicate by child name.
		seen := make(map[string]bool)
		for _, sec := range members {
			if sec.Slug == groupSlug {
				continue
			}
			name := childName(sec.Slug, groupSlug)
			if seen[name] {
				continue
			}
			seen[name] = true
			_, _ = fmt.Fprintf(w, "  - %s\n", name)
			if firstChildSlug == "" {
				firstChildSlug = sec.Slug
			}
		}
	}

	if firstChildSlug != "" {
		printExample(w, "k6 x docs "+slugToArgs(firstChildSlug))
	}
}

// printBestPractices reads and prints the best_practices.md file from the cache.
func printBestPractices(afs fsext.Fs, w io.Writer, cacheDir, version string) error {
	path := filepath.Join(cacheDir, "best_practices.md")
	data, err := fsext.ReadFile(afs, path)
	if err != nil {
		return fmt.Errorf("read best practices: %w", err)
	}
	content := Transform(string(data), version)
	_, _ = fmt.Fprint(w, content)
	if !strings.HasSuffix(content, "\n") {
		_, _ = fmt.Fprintln(w)
	}
	return nil
}

// readMarkdown reads a markdown file from the cache directory.
func readMarkdown(afs fsext.Fs, cacheDir, relPath string) string {
	path := filepath.Join(cacheDir, "markdown", relPath)
	data, err := fsext.ReadFile(afs, path)
	if err != nil {
		return ""
	}
	return string(data)
}

// readAndTransform reads a markdown file and applies runtime transforms.
func readAndTransform(afs fsext.Fs, cacheDir, relPath, version string) string {
	raw := readMarkdown(afs, cacheDir, relPath)
	if raw == "" {
		return ""
	}
	return Transform(raw, version)
}
