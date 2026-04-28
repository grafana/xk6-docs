package cli

import (
	"errors"
	"regexp"
	"runtime/debug"

	"github.com/grafana/xk6-docs/docs"
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
			return docs.VersionWildcard(dep.Version), nil
		}
	}

	return "", errors.New("go.k6.io/k6 dependency not found in build info")
}
