package docs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
)

func newCmd(gs *state.GlobalState) *cobra.Command {
	return newDocsCmd(gs)
}

func newDocsCmd(gs *state.GlobalState) *cobra.Command {
	var opts docsOpts

	cmd := &cobra.Command{
		Use:   "docs [topic] [subtopic...]",
		Short: "Print k6 documentation",
		Long: `Offline k6 documentation in the terminal.

Auto-downloads docs matching your k6 version on first run, then serves
from cache. Topics resolve from space-separated args (e.g. "http get").
Use search to find topics quickly.`,
		Example: `  k6 x docs                        Show table of contents
  k6 x docs http                   Read the HTTP module docs
  k6 x docs http get               Read a specific topic
  k6 x docs search websocket       Search across all docs
  k6 x docs best-practices         Show best practices guide`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDocs(gs, cmd, args, &opts)
		},
	}

	completionTopics := newTopicCompletion(gs, &opts)
	cmd.ValidArgsFunction = completionTopics

	cmd.PersistentFlags().StringVar(&opts.version, "version", "", "Override k6 version for docs lookup")
	cmd.PersistentFlags().StringVar(&opts.cacheDir, "cache-dir", "", "Override cache directory")
	cmd.PersistentFlags().IntVar(&opts.depth, "depth", 0, "Override subtopic depth (default 1)")
	cmd.PersistentFlags().BoolVarP(&opts.pager, "pager", "p", false, "Display with pager")

	_ = cmd.RegisterFlagCompletionFunc("version", cobra.NoFileCompletions)
	_ = cmd.RegisterFlagCompletionFunc("cache-dir", completionDirs)
	_ = cmd.RegisterFlagCompletionFunc("depth", cobra.NoFileCompletions)

	searchCmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search documentation",
		Long:  "Fuzzy search across all topics (case-insensitive, ignores punctuation).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(gs, cmd, args, &opts)
		},
	}
	searchCmd.ValidArgsFunction = completionTopics
	cmd.AddCommand(searchCmd)

	skillCmd := &cobra.Command{
		Use:   "skill [directory]",
		Short: "Install the agent skill for AI coding tools",
		Long: `Install the k6 docs agent skill into a directory.

Without arguments, shows a table of supported agents and their skill directories.
With a directory argument, installs the skill files there.`,
		Example: `  k6 x docs skill                     Show supported agents
  k6 x docs skill ~/.claude/skills    Install for Claude Code
  k6 x docs skill ~/.agents/skills    Install for Cursor, Codex, etc.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkill(gs.FS, cmd.OutOrStdout(), gs.Stdout.IsTTY, gs.CmdArgs[0], args)
		},
	}
	skillCmd.ValidArgsFunction = completionDirs
	cmd.AddCommand(skillCmd)

	return cmd
}

type docsOpts struct {
	version  string
	cacheDir string
	depth    int
	pager    bool
}

// runCtx holds the common state prepared by prepareRun for both docs and search.
type runCtx struct {
	env   *docsEnv
	idx   *Index
	w     io.Writer
	flush func() error
}

// prepareRun handles setup shared by runDocs and runSearch:
// version/cache resolution, depth, and renderer buffering.
func prepareRun(gs *state.GlobalState, cmd *cobra.Command, opts *docsOpts) (*runCtx, error) {
	version, cacheDir, idx, err := setup(cmd.Context(), gs, opts.version, opts.cacheDir)
	if err != nil {
		return nil, err
	}

	depth := defaultDepth
	if opts.depth > 0 {
		depth = opts.depth
	}

	env := &docsEnv{FS: gs.FS, CacheDir: cacheDir, Version: version, Depth: depth}

	baseW := cmd.OutOrStdout()
	w := baseW
	var buf *bytes.Buffer

	if gs.Stdout.IsTTY && !gs.Flags.NoColor || opts.pager {
		buf = &bytes.Buffer{}
		w = buf
	}

	flush := func() error {
		if buf == nil || buf.Len() == 0 {
			return nil
		}
		if opts.pager {
			pagerCmd := gs.Env["PAGER"]
			if pagerCmd == "" {
				pagerCmd = "less -r"
			}
			parts := strings.Split(pagerCmd, " ")
			c := exec.CommandContext(cmd.Context(), parts[0], parts[1:]...) //nolint:gosec // user's $PAGER
			c.Stdout = gs.Stdout.Writer
			c.Stderr = gs.Stderr.Writer
			stdin, err := c.StdinPipe()
			if err != nil {
				return err
			}
			if err := c.Start(); err != nil {
				return err
			}
			err = renderMarkdown(stdin, buf.String())
			_ = stdin.Close()
			if waitErr := c.Wait(); err == nil {
				err = waitErr
			}
			return err
		}
		return renderMarkdown(gs.Stdout.Writer, buf.String())
	}

	return &runCtx{env: env, idx: idx, w: w, flush: flush}, nil
}

func runSearch(gs *state.GlobalState, cmd *cobra.Command, args []string, opts *docsOpts) error {
	rc, err := prepareRun(gs, cmd, opts)
	if err != nil {
		return err
	}
	printSearch(rc.env, rc.w, rc.idx, args)
	return rc.flush()
}

func runDocs(gs *state.GlobalState, cmd *cobra.Command, args []string, opts *docsOpts) error {
	rc, err := prepareRun(gs, cmd, opts)
	if err != nil {
		return err
	}
	logMode(gs, gs.Stdout.IsTTY)
	if err := showDocs(rc.env, rc.w, rc.idx, args); err != nil {
		return err
	}
	return rc.flush()
}

func logMode(gs *state.GlobalState, isTTY bool) {
	if gs == nil {
		return
	}
	if isTTY {
		gs.Logger.Debug("docs: interactive mode (stdout is TTY)")
	} else {
		gs.Logger.Debug("docs: agent mode (stdout is not a TTY)")
	}
}

// setup resolves the version, ensures docs are cached, and loads the index.
// It checks flags, then env vars, then auto-detection for both version and
// cache directory.
func setup(
	ctx context.Context, gs *state.GlobalState, versionFlag, cacheDirFlg string,
) (version, cacheDir string, idx *Index, err error) {
	version = versionFlag
	if version == "" {
		version = gs.Env["K6_DOCS_VERSION"]
	}
	if version == "" {
		version, err = DetectK6Version()
		if err != nil {
			return "", "", nil, fmt.Errorf("detect k6 version: %w", err)
		}
	}

	version = MapToWildcard(version)

	cacheDir = cacheDirFlg
	if cacheDir == "" {
		cacheDir = gs.Env["K6_DOCS_CACHE_DIR"]
	}

	if cacheDir == "" {
		cacheDir, err = EnsureDocs(ctx, gs.FS, gs.Env, version, http.DefaultClient)
		if err != nil {
			return "", "", nil, fmt.Errorf("ensure docs: %w", err)
		}
	}

	idx, err = LoadIndex(gs.FS, cacheDir)
	if err != nil {
		return "", "", nil, fmt.Errorf("load index: %w", err)
	}

	return version, cacheDir, idx, nil
}
