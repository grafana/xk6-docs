package docs

import "strings"

const jsAPISlug = "javascript-api"

// normalizeArgs flattens slash-separated segments in args.
// Shared by both slug resolution and search term preparation.
func normalizeArgs(args []string) []string {
	var flat []string
	for _, a := range args {
		for part := range strings.SplitSeq(a, "/") {
			if part != "" {
				flat = append(flat, part)
			}
		}
	}
	return flat
}

// resolveWithLookup converts CLI args into a canonical documentation slug.
// When exists is non-nil, it disambiguates javascript-api children that
// may or may not carry the k6- prefix.
//
// The k6- prefix fallback is handled in a single place (withK6Prefix)
// and applies to all javascript-api slugs regardless of how they were
// constructed (shorthand or full path).
func resolveWithLookup(args []string, exists func(string) bool) string {
	if len(args) == 0 {
		return ""
	}

	args = normalizeArgs(args)

	var slug string

	direct := strings.Join(args, "/")
	firstSeg, _, _ := strings.Cut(args[0], "/")
	if exists != nil && (exists(direct) || exists(firstSeg)) {
		slug = direct
	} else {
		// JS API module shortcut: strip k6- prefix (if present) and
		// build the base javascript-api/ slug. The k6- prefix fallback
		// below handles re-adding it when needed.
		name := strings.TrimPrefix(args[0], "k6-")
		rest := args[1:]
		parts := append([]string{name}, rest...)
		slug = jsAPISlug + "/" + strings.Join(parts, "/")
	}

	slug = withK6Prefix(slug, exists)
	return withParentFallback(slug, exists)
}

// withK6Prefix tries inserting "k6-" on the second segment of a
// javascript-api/ slug. If the original slug already exists, it is
// returned as-is (existing docs are prioritized over the k6- form).
// Without a lookup function, it defaults to the k6-prefixed form
// since most JS API modules use it.
func withK6Prefix(slug string, exists func(string) bool) string {
	prefix := jsAPISlug + "/"
	if !strings.HasPrefix(slug, prefix) {
		return slug
	}
	rest := slug[len(prefix):]
	if strings.HasPrefix(rest, "k6-") {
		return slug
	}

	candidate := prefix + "k6-" + rest

	if exists == nil {
		return candidate
	}
	if exists(slug) {
		return slug
	}
	// Return k6-prefixed form: either it exists, or it's the better
	// default for further fallbacks (most JS API modules use k6-).
	return candidate
}

// withParentFallback retries a slug by prepending the parent segment name
// to the last segment. This handles children whose actual slug carries a
// redundant parent prefix (e.g. parent/parent-child).
func withParentFallback(slug string, exists func(string) bool) string {
	if exists == nil || exists(slug) {
		return slug
	}
	i := strings.LastIndex(slug, "/")
	if i < 0 {
		return slug
	}
	parent := slug[:i]
	child := slug[i+1:]
	var parentName string
	if j := strings.LastIndex(parent, "/"); j >= 0 {
		parentName = parent[j+1:]
	} else {
		parentName = parent
	}
	candidate := parent + "/" + parentName + "-" + child
	if exists(candidate) {
		return candidate
	}
	return slug
}
