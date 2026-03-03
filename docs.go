package docs

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"go.k6.io/k6/lib/fsext"
)

// docsEnv bundles the context needed for reading and transforming docs.
type docsEnv struct {
	FS       fsext.Fs
	CacheDir string
	Version  string
	Depth    int
}

func (env *docsEnv) readAndTransform(relPath string) string {
	raw := readMarkdown(env.FS, env.CacheDir, relPath)
	if raw == "" {
		return ""
	}
	return Transform(raw, env.Version)
}

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

// printTree prints sections as indented bullets, recursing into children up to depth levels.
func printTree(w io.Writer, idx *Index, items []*Section, parentSlug, indent string, depth int) {
	if depth < 1 {
		return
	}
	for _, item := range items {
		name := childName(item.Slug, parentSlug)
		_, _ = fmt.Fprintf(w, "%s- %s\n", indent, name)
		if depth > 1 {
			children := idx.Children(item.Slug)
			printTree(w, idx, children, item.Slug, indent+"  ", depth-1)
		}
	}
}

// printSubtopics prints a subtopics block: bold header, bullet list, and example hint.
func printSubtopics(w io.Writer, idx *Index, path string, children []*Section, parentSlug string, depth int) {
	_, _ = fmt.Fprintf(w, "**%s subtopics:**\n", path)
	printTree(w, idx, children, parentSlug, "", depth)
	printExample(w, fmt.Sprintf("k6 x docs %s/<subtopic>", path))
}

// showDocs resolves args to a topic and prints it.
func showDocs(env *docsEnv, w io.Writer, idx *Index, args []string) error {
	if len(args) == 0 {
		printTOC(w, idx, env.Version, env.Depth)
		return nil
	}

	if args[0] == "best-practices" {
		return printBestPractices(env, w)
	}

	slug := ResolveWithLookup(args, func(s string) bool {
		_, ok := idx.Lookup(s)
		return ok
	})

	sec, ok := idx.Lookup(slug)
	if !ok {
		return fmt.Errorf("topic not found: %s", strings.Join(args, " "))
	}

	printSection(env, w, idx, sec)
	return nil
}

// printTOC prints the table of contents with subtopics up to depth levels.
func printTOC(w io.Writer, idx *Index, version string, depth int) {
	_, _ = fmt.Fprintf(w, "# k6 %s\n", version)
	printTree(w, idx, idx.TopLevel(), "", "", depth)
	printExample(w, "k6 x docs <topic>")
}

// printSection prints a section's markdown content, read from the cache dir.
// If the section has children, a subtopics footer is appended.
func printSection(env *docsEnv, w io.Writer, idx *Index, section *Section) {
	content := strings.TrimSpace(env.readAndTransform(section.RelPath))
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

		printSubtopics(w, idx, path, children, section.Slug, env.Depth)
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
func printSearch(env *docsEnv, w io.Writer, idx *Index, term string) {
	readContent := func(slug string) string {
		sec, ok := idx.Lookup(slug)
		if !ok {
			return ""
		}
		return env.readAndTransform(sec.RelPath)
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
func printBestPractices(env *docsEnv, w io.Writer) error {
	path := filepath.Join(env.CacheDir, "best_practices.md")
	data, err := fsext.ReadFile(env.FS, path)
	if err != nil {
		return fmt.Errorf("read best practices: %w", err)
	}
	content := Transform(string(data), env.Version)
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
