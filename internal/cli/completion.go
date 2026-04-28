package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime/debug"
	"strings"

	"github.com/grafana/xk6-docs/docs"
	"github.com/spf13/cobra"
)

// completionDirs is a ValidArgsFunction that returns directory completions.
func completionDirs(
	_ *cobra.Command, _ []string, _ string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
}

// setupForCompletion loads the index from the local cache without network I/O.
// Returns the index and the resolved version. When the index is nil but
// version is non-empty, the cache is missing for that version.
func setupForCompletion(
	ctx context.Context, env map[string]string, opts *docsOpts,
) (*docs.Index, string) {
	version := opts.version
	if version == "" {
		version = env["K6_DOCS_VERSION"]
	}
	if version == "" {
		v, err := detectK6Version(debug.ReadBuildInfo)
		if err != nil {
			return nil, ""
		}
		version = v
	}

	version = docs.VersionWildcard(version)

	base := cmp.Or(opts.cacheDir, env["K6_DOCS_CACHE_DIR"], baseCacheDir(env))
	if base == "" {
		return nil, ""
	}

	cat := docs.NewCatalog(docs.WithCacheDir(base), docs.WithLocalOnly())
	idx, err := cat.Index(ctx, version)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, version // cache missing: active help
		}
		return nil, "" // bad data: silent
	}

	return idx, version
}

func completionTopicArgs(
	cmd *cobra.Command, idx *docs.Index, args []string, toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	if idx == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if len(args) == 0 {
		return completionFirstArg(cmd, idx, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	return completionDeeper(idx, args, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completionFirstArg(cmd *cobra.Command, idx *docs.Index, toComplete string) []cobra.Completion {
	prefix := strings.ToLower(toComplete)
	var comps []cobra.Completion

	for _, sub := range cmd.Commands() {
		if !strings.HasPrefix(sub.Name(), prefix) {
			continue
		}
		comps = append(comps, fmt.Sprintf("%s\t%s", sub.Name(), sub.Short))
	}

	topLevel := make(map[string]bool)
	for _, sec := range idx.TopLevel() {
		if !strings.HasPrefix(sec.Slug, prefix) {
			continue
		}
		comps = append(comps, sec.Slug)
		topLevel[sec.Slug] = true
	}

	jsAPI, ok := idx.Lookup(jsAPISlug)
	if ok {
		seen := make(map[string]bool)
		for _, childSlug := range jsAPI.Children {
			if _, ok := idx.Lookup(childSlug); !ok {
				continue
			}
			short := strings.TrimPrefix(childName(childSlug, jsAPISlug), "k6-")
			if topLevel[short] || seen[short] {
				continue
			}
			seen[short] = true
			if !strings.HasPrefix(strings.ToLower(short), prefix) {
				continue
			}
			comps = append(comps, short)
		}
	}

	bp := "best-practices"
	if strings.HasPrefix(bp, prefix) {
		comps = append(comps, bp)
	}

	return comps
}

func completionDeeper(idx *docs.Index, args []string, toComplete string) []cobra.Completion {
	exists := func(s string) bool { _, ok := idx.Lookup(s); return ok }
	slug := resolveWithLookup(args, exists)

	sec, ok := idx.Lookup(slug)
	if !ok || len(sec.Children) == 0 {
		return nil
	}

	prefix := strings.ToLower(toComplete)
	seen := make(map[string]bool)
	var comps []cobra.Completion

	for _, childSlug := range sec.Children {
		_, ok := idx.Lookup(childSlug)
		if !ok {
			continue
		}
		name := childName(childSlug, slug)
		if seen[name] {
			continue
		}
		seen[name] = true
		if !strings.HasPrefix(strings.ToLower(name), prefix) {
			continue
		}
		comps = append(comps, name)
	}

	return comps
}
