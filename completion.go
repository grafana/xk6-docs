package docs

import (
	"strings"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

func newTopicCompletion(
	gs *state.GlobalState, opts *docsOpts,
) func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		idx := setupForCompletion(gs, opts)
		if idx == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completionTopicArgs(idx, args, toComplete)
	}
}

func completionDirs(
	_ *cobra.Command, _ []string, _ string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
}

func setupForCompletion(gs *state.GlobalState, opts *docsOpts) *Index {
	version := opts.version
	if version == "" {
		version = gs.Env["K6_DOCS_VERSION"]
	}
	if version == "" {
		v, err := DetectK6Version()
		if err != nil {
			return nil
		}
		version = v
	}

	version = MapToWildcard(version)

	cacheDir := opts.cacheDir
	if cacheDir == "" {
		cacheDir = gs.Env["K6_DOCS_CACHE_DIR"]
	}
	if cacheDir == "" {
		dir, err := CacheDir(gs.Env, version)
		if err != nil {
			return nil
		}
		cacheDir = dir
	}

	if !IsCached(gs.FS, gs.Env, version) {
		if !dirExists(gs.FS, cacheDir) {
			return nil
		}
	}

	idx, err := LoadIndex(gs.FS, cacheDir)
	if err != nil {
		return nil
	}

	return idx
}

func dirExists(afs fsext.Fs, path string) bool {
	info, err := afs.Stat(path)
	return err == nil && info.IsDir()
}

func completionTopicArgs(
	idx *Index, args []string, toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	if idx == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if len(args) == 0 {
		return completionFirstArg(idx, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	return completionDeeper(idx, args, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completionFirstArg(idx *Index, toComplete string) []cobra.Completion {
	prefix := strings.ToLower(toComplete)
	var comps []cobra.Completion

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
		for _, childSlug := range jsAPI.Children {
			if _, ok := idx.Lookup(childSlug); !ok {
				continue
			}
			short := strings.TrimPrefix(childName(childSlug, jsAPISlug), "k6-")
			if topLevel[short] {
				continue
			}
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

func completionDeeper(idx *Index, args []string, toComplete string) []cobra.Completion {
	exists := func(s string) bool { _, ok := idx.Lookup(s); return ok }
	slug := ResolveWithLookup(args, exists)

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
