package docs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
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
		Long:  "Access k6 documentation from the command line.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDocs(gs, cmd, args, &opts)
		},
	}

	cmd.Flags().BoolVar(&opts.list, "list", false, "List subtopics instead of showing content")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Print all documentation")
	cmd.PersistentFlags().StringVar(&opts.version, "version", "", "Override k6 version for docs lookup")
	cmd.PersistentFlags().StringVar(&opts.cacheDir, "cache-dir", "", "Override cache directory")

	searchCmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search documentation",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(gs, cmd, args, &opts)
		},
	}
	cmd.AddCommand(searchCmd)

	return cmd
}

type docsOpts struct {
	list     bool
	all      bool
	version  string
	cacheDir string
}

func runSearch(gs *state.GlobalState, cmd *cobra.Command, args []string, opts *docsOpts) error {
	version, cacheDir, idx, err := setup(gs, opts.version, opts.cacheDir)
	if err != nil {
		return err
	}

	cfg, cfgErr := loadConfig(gs.FS, gs.Env)
	if cfgErr != nil {
		gs.Logger.Warnf("docs: ignoring invalid config: %v", cfgErr)
	}

	isTTY := gs.Stdout.IsTTY
	baseW := cmd.OutOrStdout()
	var buf *bytes.Buffer
	w := baseW

	if cfg.Renderer != "" && isTTY {
		buf = &bytes.Buffer{}
		w = buf
	}

	term := strings.Join(args, " ")
	printSearch(gs.FS, w, idx, term, cacheDir, version)
	return pipeRenderer(cmd.Context(), buf, gs.Stdout.Writer, baseW, gs.Stderr, cfg.Renderer)
}

func runDocs(gs *state.GlobalState, cmd *cobra.Command, args []string, opts *docsOpts) error {
	version, cacheDir, idx, err := setup(gs, opts.version, opts.cacheDir)
	if err != nil {
		return err
	}

	isTTY := gs.Stdout.IsTTY
	logMode(gs, isTTY)

	cfg, cfgErr := loadConfig(gs.FS, gs.Env)
	if cfgErr != nil && gs != nil {
		gs.Logger.Warnf("docs: ignoring invalid config: %v", cfgErr)
	}

	baseW := cmd.OutOrStdout()
	var buf *bytes.Buffer
	w := baseW

	if cfg.Renderer != "" && isTTY {
		buf = &bytes.Buffer{}
		w = buf
	}

	if opts.all {
		printAll(gs.FS, w, idx, cacheDir, version)
		return pipeRenderer(cmd.Context(), buf, gs.Stdout.Writer, baseW, gs.Stderr, cfg.Renderer)
	}

	if opts.list && len(args) == 0 {
		printTopLevelList(w, idx)
		return pipeRenderer(cmd.Context(), buf, gs.Stdout.Writer, baseW, gs.Stderr, cfg.Renderer)
	}

	if len(args) == 0 {
		printTOC(w, idx, version)
		return pipeRenderer(cmd.Context(), buf, gs.Stdout.Writer, baseW, gs.Stderr, cfg.Renderer)
	}

	if args[0] == "best-practices" {
		if err := printBestPractices(gs.FS, w, cacheDir, version); err != nil {
			return err
		}
		return pipeRenderer(cmd.Context(), buf, gs.Stdout.Writer, baseW, gs.Stderr, cfg.Renderer)
	}

	slug := ResolveWithLookup(args, func(s string) bool {
		_, ok := idx.Lookup(s)
		return ok
	})

	sec, ok := idx.Lookup(slug)
	if !ok {
		return fmt.Errorf("topic not found: %s", strings.Join(args, " "))
	}

	if opts.list {
		printList(w, idx, slug)
		return pipeRenderer(cmd.Context(), buf, gs.Stdout.Writer, baseW, gs.Stderr, cfg.Renderer)
	}

	printSection(gs.FS, w, idx, sec, cacheDir, version)
	return pipeRenderer(cmd.Context(), buf, gs.Stdout.Writer, baseW, gs.Stderr, cfg.Renderer)
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

func pipeRenderer(
	ctx context.Context, buf *bytes.Buffer, stdout, fallback, stderr io.Writer, renderer string,
) error {
	if buf == nil || buf.Len() == 0 {
		return nil
	}

	raw := buf.Bytes()

	parts := strings.Fields(renderer)
	if len(parts) == 0 {
		_, err := fallback.Write(raw)
		return err
	}

	bin, err := exec.LookPath(parts[0])
	if err != nil {
		_, writeErr := fallback.Write(raw)
		return writeErr
	}

	if err := runRenderer(ctx, bin, parts[1:], bytes.NewReader(raw), stdout, stderr); err != nil {
		_, writeErr := fallback.Write(raw)
		return writeErr
	}

	return nil
}

func runRenderer(
	ctx context.Context, bin string, args []string, stdin io.Reader, stdout, stderr io.Writer,
) error {
	//nolint:gosec // G204 has no sanitizer support (unlike G304). bin is validated via exec.LookPath.
	rc := exec.CommandContext(ctx, filepath.Clean(bin), args...)
	rc.Stdin = stdin
	rc.Stdout = stdout
	rc.Stderr = stderr

	return rc.Run()
}

// setup resolves the version, ensures docs are cached, and loads the index.
// It checks flags, then env vars, then auto-detection for both version and
// cache directory.
func setup(gs *state.GlobalState, versionFlag, cacheDirFlg string) (version, cacheDir string, idx *Index, err error) {
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

	cacheDir = cacheDirFlg
	if cacheDir == "" {
		cacheDir = gs.Env["K6_DOCS_CACHE_DIR"]
	}

	if cacheDir == "" {
		cacheDir, err = EnsureDocs(gs.FS, gs.Env, version, http.DefaultClient)
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
