package docs

import "strings"

// Resolve converts CLI args into a canonical documentation slug.
// It always assumes the k6- prefix for Rule 3 (JS API shortcuts).
// Use [ResolveWithLookup] when an index is available to handle
// slugs that don't carry the k6- prefix (e.g. jslib, crypto).
func Resolve(args []string) string {
	return ResolveWithLookup(args, nil)
}

// ResolveWithLookup converts CLI args into a canonical documentation slug.
// When exists is non-nil, Rule 3 uses it to disambiguate javascript-api
// children that may or may not carry the k6- prefix.
//
// If the user typed "k6-http", it always resolves to javascript-api/k6-http.
// If the user typed a bare name like "jslib" or "crypto", the function tries
// the unprefixed slug first (javascript-api/crypto), then falls back to the
// k6-prefixed form (javascript-api/k6-crypto). This handles pages like jslib,
// crypto, init-context, and error-codes that don't use the k6- prefix.
func ResolveWithLookup(args []string, exists func(string) bool) string {
	if len(args) == 0 {
		return ""
	}

	// Normalize: split all args on "/" so "browser/elementhandle"
	// resolves the same way as "browser" "elementhandle".
	var flat []string
	for _, a := range args {
		for part := range strings.SplitSeq(a, "/") {
			if part != "" {
				flat = append(flat, part)
			}
		}
	}
	args = flat

	// Rule 2: first word matches a known category prefix → join all words.
	if isCategory(args[0]) {
		return strings.Join(args, "/")
	}

	// Rule 3: JS API module shortcut.
	hasK6Prefix := strings.HasPrefix(args[0], "k6-")
	name := strings.TrimPrefix(args[0], "k6-")
	rest := args[1:]
	parts := append([]string{name}, rest...)
	prefixed := "javascript-api/k6-" + strings.Join(parts, "/")

	if exists == nil || hasK6Prefix {
		// No lookup available, or user explicitly typed k6- prefix.
		return prefixed
	}

	// User typed a bare name: try unprefixed first (exact match), then k6- prefixed.
	unprefixed := "javascript-api/" + strings.Join(parts, "/")
	if exists(unprefixed) {
		return unprefixed
	}

	if exists(prefixed) {
		return prefixed
	}

	// Neither found — try parent-prefix fallback, then return prefixed as default.
	return withParentFallback(prefixed, exists)
}

// withParentFallback retries a slug by prepending the parent segment name
// to the last segment. This handles children whose actual slug carries a
// redundant parent prefix (e.g. cookiejar/cookiejar-clear).
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
