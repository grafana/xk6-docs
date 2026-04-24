package docs

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

func newTopicCompletion(
	gs *state.GlobalState, opts *docsOpts,
) func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		idx, version := setupForCompletion(gs, opts)
		if idx == nil && version != "" && gs.Flags.AutoExtensionResolution {
			msg := fmt.Sprintf("Press ENTER to load the k6 %s docs for completions to work.", version)
			comps := cobra.AppendActiveHelp(nil, msg)
			return comps, cobra.ShellCompDirectiveNoFileComp
		}
		if idx == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completionTopicArgs(cmd, idx, args, toComplete)
	}
}

func completionDirs(
	_ *cobra.Command, _ []string, _ string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
}

// setupForCompletion loads the index from the local cache without network I/O.
// Returns the index and the resolved version. When the index is nil but
// version is non-empty, the cache is missing for that version.
func setupForCompletion(gs *state.GlobalState, opts *docsOpts) (*Index, string) {
	version := opts.version
	if version == "" {
		version = gs.Env["K6_DOCS_VERSION"]
	}
	if version == "" {
		v, err := detectK6Version(debug.ReadBuildInfo)
		if err != nil {
			return nil, ""
		}
		version = v
	}

	version = MapToWildcard(version)

	docsCacheDir := opts.cacheDir
	if docsCacheDir == "" {
		docsCacheDir = gs.Env["K6_DOCS_CACHE_DIR"]
	}
	if docsCacheDir == "" {
		dir, err := cacheDir(gs.Env, version)
		if err != nil {
			return nil, ""
		}
		docsCacheDir = dir
	}

	if !dirExists(gs.FS, docsCacheDir) {
		return nil, version
	}

	idx, err := LoadIndex(gs.FS, docsCacheDir)
	if err != nil {
		return nil, ""
	}

	return idx, version
}

func dirExists(afs fsext.Fs, path string) bool {
	info, err := afs.Stat(path)
	return err == nil && info.IsDir()
}

func completionTopicArgs(
	cmd *cobra.Command, idx *Index, args []string, toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	if idx == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if len(args) == 0 {
		return completionFirstArg(cmd, idx, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	return completionDeeper(idx, args, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completionFirstArg(cmd *cobra.Command, idx *Index, toComplete string) []cobra.Completion {
	prefix := strings.ToLower(toComplete)
	var comps []cobra.Completion

	// Add subcommands (hidden from cobra to suppress them when cache
	// is missing, so we add them here when the cache is ready).
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

func completionDeeper(idx *Index, args []string, toComplete string) []cobra.Completion {
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
