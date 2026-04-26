package docs

import "strings"

// VersionWildcard maps a semantic k6 version to the wildcard directory name
// used for documentation lookups (for example "v1.5.0" becomes "v1.5.x").
//
// Pre-release suffixes (after "-") and build metadata (after "+") are stripped
// before the replacement. A "v" prefix is added to the result if missing,
// since k6-docs uses v-prefixed directory names.
//
// An empty input returns an empty string. If the cleaned input has fewer than
// two dots, the original input is returned unchanged.
func VersionWildcard(version string) string {
	if version == "" {
		return ""
	}

	clean := version
	if idx := strings.IndexAny(clean, "-+"); idx != -1 {
		clean = clean[:idx]
	}

	lastDot := strings.LastIndex(clean, ".")
	if lastDot == -1 {
		return version
	}

	prefix := clean[:lastDot]
	if !strings.Contains(prefix, ".") {
		return version
	}

	result := prefix + ".x"
	if !strings.HasPrefix(result, "v") {
		result = "v" + result
	}
	return result
}
