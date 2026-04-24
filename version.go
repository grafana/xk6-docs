package docs

import (
	"errors"
	"regexp"
	"runtime/debug"
	"strings"
)

// detectK6Version reads build info using the provided function and returns the
// wildcard-mapped version of the go.k6.io/k6 dependency.
var k6ModuleRe = regexp.MustCompile(`^go\.k6\.io/k6(/v[1-9][0-9]*)?$`)

func detectK6Version(readBuildInfo func() (*debug.BuildInfo, bool)) (string, error) {
	info, ok := readBuildInfo()
	if !ok {
		return "", errors.New("build info unavailable")
	}

	for _, dep := range info.Deps {
		if k6ModuleRe.MatchString(dep.Path) {
			return MapToWildcard(dep.Version), nil
		}
	}

	return "", errors.New("go.k6.io/k6 dependency not found in build info")
}

// MapToWildcard replaces the last dot-separated segment of a version string
// with "x" to produce a wildcard directory name for k6-docs lookups.
// For example, "v1.5.0" becomes "v1.5.x" and "v0.55.2-rc.1" becomes "v0.55.x".
//
// Pre-release suffixes (after "-") and build metadata (after "+") are stripped
// before the replacement. A "v" prefix is added to the result if missing, since
// the k6-docs repository uses v-prefixed directory names (e.g. "v1.6.x").
//
// If the version is empty, an empty string is returned. If the cleaned version
// contains fewer than two dots, the original version is returned unchanged
// (no wildcard replacement or prefix normalization is applied).
func MapToWildcard(version string) string {
	if version == "" {
		return ""
	}

	// Strip pre-release (-...) and build metadata (+...) first.
	// Find the earliest occurrence of either '-' or '+' after the version prefix.
	clean := version
	if idx := strings.IndexAny(clean, "-+"); idx != -1 {
		clean = clean[:idx]
	}

	// Find the last dot to replace patch with "x".
	lastDot := strings.LastIndex(clean, ".")
	if lastDot == -1 {
		return version
	}

	// Ensure there are at least two dots (major.minor.patch).
	prefix := clean[:lastDot]
	if !strings.Contains(prefix, ".") {
		return version
	}

	result := prefix + ".x"

	// Ensure the "v" prefix is present. The k6-docs repo always uses
	// v-prefixed directory names like "v1.6.x".
	if !strings.HasPrefix(result, "v") {
		result = "v" + result
	}

	return result
}

// DetectK6Version is a convenience wrapper that uses the real debug.ReadBuildInfo.
func DetectK6Version() (string, error) {
	return detectK6Version(debug.ReadBuildInfo)
}
