package docs

import (
	"regexp"
	"strings"
)

var (
	reShared     = regexp.MustCompile(`\{\{<\s*docs/shared\s+source="k6"\s+lookup="([^"]+)".*?>\}\}`)
	reCodeTag    = regexp.MustCompile(`\{\{<\s*/?\s*code\s*>\}\}`)
	reAdmonition = regexp.MustCompile(
		`(?s)\{\{<\s*admonition\s+type="([^"]+)"\s*>\}\}\s*\n(.*?)\n\s*\{\{<\s*/admonition\s*>\}\}`,
	)
	reSection      = regexp.MustCompile(`\{\{<\s*/?\s*section\b[^>]*>\}\}`)
	reAnyShortcode = regexp.MustCompile(`\{\{<\s*/?\s*[^>]+>\}\}`)
	reComponentTag = regexp.MustCompile(`</?[A-Z][a-z][a-zA-Z]*[^>]*>`) // <Glossary>, </DescriptionList>, etc.
	reBrTag        = regexp.MustCompile(`<br\s*/?>`)                    // <br/>, <br />, <br>
	reHTMLComment  = regexp.MustCompile(`<!--[\s\S]*?-->`)
	reExtraNewline = regexp.MustCompile(`\n{3,}`)

	// reImageLink matches markdown image links: ![alt](url)
	reImageLink = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	// reMarkdownLink matches markdown links: [text](url)
	// The text portion allows one level of nested brackets for cases like [get(url, [params])](url).
	reMarkdownLink = regexp.MustCompile(`\[((?:[^\[\]]|\[[^\]]*\])*)\]\([^)]+\)`)

	// reInternalLink matches markdown links pointing to Grafana k6 docs.
	// Link text may contain brackets (e.g., "get(url, [params])"), so we
	// match greedily up to "](https://grafana.com/docs/k6/".
	// Captures: [1]=link text, [2]=path after /docs/k6/vX.Y.Z/
	reInternalLink = regexp.MustCompile(`\[((?:[^\[\]]|\[[^\]]*\])*)\]\(https://grafana\.com/docs/k6/v[^/]+/([^)]*)\)`)
)

// PrepareTransform resolves docs/shared shortcodes using the shared content
// map. This runs at bundle build time because shared content files are not
// shipped in the bundle.
func PrepareTransform(content string, sharedContent map[string]string) string {
	if content == "" {
		return ""
	}

	return reShared.ReplaceAllStringFunc(content, func(match string) string {
		m := reShared.FindStringSubmatch(match)
		if m == nil || sharedContent == nil {
			return ""
		}
		raw, ok := sharedContent[m[1]]
		if !ok {
			return ""
		}
		return StripFrontmatter(raw)
	})
}

// Transform applies markdown cleanup to content. It handles all pure text
// transforms (shortcode stripping, admonition conversion, link stripping,
// frontmatter removal, whitespace normalization). The pipeline runs in a
// fixed order:
//  1. Strip code tags
//  2. Convert admonitions to blockquotes
//  3. Strip section tags
//  4. Strip remaining shortcodes
//     4a. Strip React/MDX component tags (PascalCase)
//     4b. Strip <br/> tags
//  5. Replace <K6_VERSION> with version
//  6. Convert internal docs links to plain text
//     6a. Strip remaining markdown image links
//     6b. Strip remaining markdown links
//  7. Strip HTML comments
//  8. Strip YAML frontmatter
//  9. Normalize whitespace
func Transform(content, version string) string {
	if content == "" {
		return ""
	}

	s := content

	// 1. Strip code tags (keep content between them).
	s = reCodeTag.ReplaceAllString(s, "")

	// 3. Convert admonitions to blockquotes.
	s = reAdmonition.ReplaceAllStringFunc(s, func(match string) string {
		m := reAdmonition.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		title := strings.ToUpper(m[1][:1]) + m[1][1:]
		body := strings.TrimSpace(m[2])

		lines := strings.Split(body, "\n")
		var sb strings.Builder
		first := true
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if first {
				sb.WriteString("> **" + title + ":** " + line + "\n")
				first = false
			} else {
				sb.WriteString("> " + line + "\n")
			}
		}
		return sb.String()
	})

	// 4. Strip section tags.
	s = reSection.ReplaceAllString(s, "")

	// 5. Strip remaining shortcodes.
	s = reAnyShortcode.ReplaceAllString(s, "")

	// 5a. Strip React/MDX component tags (PascalCase like <Glossary>, <DescriptionList>).
	s = reComponentTag.ReplaceAllString(s, "")

	// 5b. Strip <br/> tags.
	s = reBrTag.ReplaceAllString(s, "")

	// 6. Replace version placeholder.
	s = strings.ReplaceAll(s, "<K6_VERSION>", version)

	// 7. Convert internal docs links to plain text.
	// Links pointing to categories we ship become just the link text.
	// Links to excluded categories (extensions, set-up, etc.) keep the URL.
	s = reInternalLink.ReplaceAllStringFunc(s, func(match string) string {
		m := reInternalLink.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		linkText, path := m[1], m[2]
		// Strip trailing slash and anchor.
		clean := strings.SplitN(path, "#", 2)[0]
		clean = strings.TrimRight(clean, "/")
		if IsIncludedDocsPath(clean) {
			return linkText
		}
		return match
	})

	// 7a. Strip remaining markdown image links, keeping alt text.
	s = reImageLink.ReplaceAllString(s, "$1")

	// 7b. Strip remaining markdown links, keeping link text.
	s = reMarkdownLink.ReplaceAllString(s, "$1")

	// 8. Strip HTML comments.
	s = reHTMLComment.ReplaceAllString(s, "")

	// 9. Strip YAML frontmatter.
	s = StripFrontmatter(s)

	// 10. Normalize whitespace: collapse 3+ consecutive newlines to 2.
	s = reExtraNewline.ReplaceAllString(s, "\n\n")

	return s
}

// SplitFrontmatter splits content into the YAML block and the body after
// the closing delimiter. If no valid frontmatter is found, yaml is empty,
// body is the original content, and ok is false.
func SplitFrontmatter(content string) (yamlBlock, body string, ok bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content, false
	}

	end := strings.Index(content[4:], "\n---")
	if end == -1 {
		return "", content, false
	}

	// Skip past the closing "\n---" (4 bytes).
	cutAt := 4 + end + 4
	// Also consume the newline right after the closing "---" if present.
	if cutAt < len(content) && content[cutAt] == '\n' {
		cutAt++
	}
	return content[4 : 4+end], content[cutAt:], true
}

// StripFrontmatter removes YAML frontmatter (delimited by "---") from the
// start of content. If the content doesn't start with "---\n" or the closing
// delimiter is missing, it returns the content unchanged.
func StripFrontmatter(content string) string {
	_, body, _ := SplitFrontmatter(content)
	return body
}
