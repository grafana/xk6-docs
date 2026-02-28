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

// truncate shortens s to limit characters, appending "..." if truncated.
func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit-3] + "..."
}

// listItem is a name+description pair for aligned list rendering.
type listItem struct {
	Name        string
	Description string
}

// printAlignedList prints items as a left-aligned name+description list.
// Duplicate names (by Name) are skipped.
func printAlignedList(w io.Writer, items []listItem) {
	const indent = "- "
	seen := make(map[string]bool, len(items))

	maxWidth := 0
	for _, item := range items {
		if seen[item.Name] {
			continue
		}
		seen[item.Name] = true
		if len(item.Name) > maxWidth {
			maxWidth = len(item.Name)
		}
	}

	fmtStr := fmt.Sprintf("%s%%-%ds %%s\n", indent, maxWidth+1)

	// Reset seen for the printing pass.
	for k := range seen {
		delete(seen, k)
	}
	for _, item := range items {
		if seen[item.Name] {
			continue
		}
		seen[item.Name] = true
		_, _ = fmt.Fprintf(w, fmtStr, item.Name, truncate(item.Description, 80))
	}
}

// printTOC prints the table of contents grouped by category.
func printTOC(w io.Writer, idx *Index, version string) {
	_, _ = fmt.Fprintf(w, "k6 Documentation (%s)\n", version)
	_, _ = fmt.Fprintln(w, "Use: k6 x docs <topic>")

	topLevel := idx.TopLevel()

	for _, cat := range topLevel {
		_, _ = fmt.Fprintf(w, "\n## %s\n", cat.Title)

		children := idx.Children(cat.Slug)
		if len(children) == 0 {
			// Show the category itself if it has no children.
			_, _ = fmt.Fprintf(w, "- %s %s\n", childName(cat.Slug, ""), truncate(cat.Description, 80))
			continue
		}

		items := make([]listItem, 0, len(children))
		for _, child := range children {
			items = append(items, listItem{
				Name:        childName(child.Slug, cat.Slug),
				Description: child.Description,
			})
		}
		printAlignedList(w, items)
		_, _ = fmt.Fprintf(w, "\n  → Usage: k6 x docs %s <topic>\n", cat.Slug)
	}
}

// printSection prints a section's markdown content, read from the cache dir.
// If the section has children, a subtopics footer is appended.
func printSection(afs fsext.Fs, w io.Writer, idx *Index, section *Section, cacheDir, version string) {
	content := readAndTransform(afs, cacheDir, section.RelPath, version)
	if content != "" {
		_, _ = fmt.Fprint(w, content)
		if !strings.HasSuffix(content, "\n") {
			_, _ = fmt.Fprintln(w)
		}
	}

	children := idx.Children(section.Slug)
	if len(children) > 0 {
		names := make([]string, 0, len(children))
		for _, c := range children {
			names = append(names, childName(c.Slug, section.Slug))
		}

		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "---")
		_, _ = fmt.Fprintf(w, "Subtopics: %s\n", strings.Join(names, ", "))
		_, _ = fmt.Fprintf(w, "Use: k6 x docs %s <subtopic>\n", slugToArgs(section.Slug))
	}
}

// printList prints children of a section in compact format.
func printList(w io.Writer, idx *Index, slug string) {
	sec, ok := idx.Lookup(slug)
	if !ok {
		_, _ = fmt.Fprintf(w, "Topic not found: %s\n", slug)
		return
	}

	children := idx.Children(slug)

	_, _ = fmt.Fprintf(w, "%s", sec.Title)
	if sec.Description != "" {
		_, _ = fmt.Fprintf(w, " — %s", sec.Description)
	}
	_, _ = fmt.Fprintln(w)

	if len(children) == 0 {
		_, _ = fmt.Fprintln(w, "\n  (no subtopics)")
		return
	}

	items := make([]listItem, 0, len(children))
	for _, child := range children {
		items = append(items, listItem{
			Name:        childName(child.Slug, slug),
			Description: child.Description,
		})
	}
	printAlignedList(w, items)
}

// printTopLevelList lists all top-level categories with their descriptions.
func printTopLevelList(w io.Writer, idx *Index) {
	cats := idx.TopLevel()

	items := make([]listItem, 0, len(cats))
	for _, cat := range cats {
		items = append(items, listItem{
			Name:        cat.Slug,
			Description: cat.Description,
		})
	}
	printAlignedList(w, items)
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

// printSearch prints search results grouped hierarchically by topic.
func printSearch(afs fsext.Fs, w io.Writer, idx *Index, term, cacheDir, version string) {
	readContent := func(slug string) string {
		sec, ok := idx.Lookup(slug)
		if !ok {
			return ""
		}
		return readAndTransform(afs, cacheDir, sec.RelPath, version)
	}

	results := idx.Search(term, readContent)

	_, _ = fmt.Fprintf(w, "Results for %q:\n", term)

	if len(results) == 0 {
		_, _ = fmt.Fprintln(w, "\n  (no results)")
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
			// If there's a javascript-api/{key} section, this is a JS API module group.
			if members[0].Slug == "javascript-api/"+key || strings.HasPrefix(members[0].Slug, "javascript-api/"+key+"/") {
				groupSlug = "javascript-api/" + key
			}
		}

		groupSec := matched[groupSlug]

		// Print group header.
		if groupSec != nil {
			_, _ = fmt.Fprintf(w, "%s: %s\n", key, truncate(groupSec.Description, 80))
		} else {
			_, _ = fmt.Fprintf(w, "%s:\n", key)
		}

		// Collect children (items that aren't the group header itself).
		var items []listItem
		for _, sec := range members {
			if sec.Slug == groupSlug {
				continue
			}
			items = append(items, listItem{
				Name:        childName(sec.Slug, groupSlug),
				Description: sec.Description,
			})
		}
		printAlignedList(w, items)

		_, _ = fmt.Fprintln(w)
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

// printAll prints all sections sequentially.
func printAll(afs fsext.Fs, w io.Writer, idx *Index, cacheDir, version string) {
	_, _ = fmt.Fprintf(w, "k6 Documentation (%s)\n", version)

	for i := range idx.Sections {
		sec := &idx.Sections[i]
		content := readAndTransform(afs, cacheDir, sec.RelPath, version)
		if content == "" {
			continue
		}
		_, _ = fmt.Fprint(w, content)
		if !strings.HasSuffix(content, "\n") {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintln(w)
	}
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
