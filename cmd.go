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

	cmd.PersistentFlags().StringVar(&opts.version, "version", "", "Override k6 version for docs lookup")
	cmd.PersistentFlags().StringVar(&opts.cacheDir, "cache-dir", "", "Override cache directory")
	cmd.PersistentFlags().IntVar(&opts.depth, "depth", 0, "Override subtopic depth (default from config or 2)")

	searchCmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search documentation",
		Long:  "Fuzzy search across all topics (case-insensitive, ignores punctuation).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(gs, cmd, args, &opts)
		},
	}
	cmd.AddCommand(searchCmd)

	return cmd
}

type docsOpts struct {
	version  string
	cacheDir string
	depth    int
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

	env := &docsEnv{FS: gs.FS, CacheDir: cacheDir, Version: version}
	term := strings.Join(args, " ")
	printSearch(env, w, idx, term)
	return pipeRenderer(cmd.Context(), buf, gs.Stdout.Writer, baseW, gs.Stderr, cfg.Renderer)
}

func runDocs(gs *state.GlobalState, cmd *cobra.Command, args []string, opts *docsOpts) error {
	version, cacheDir, idx, err := setup(gs, opts.version, opts.cacheDir)
	if err != nil {
		return err
	}

	logMode(gs, gs.Stdout.IsTTY)

	cfg, cfgErr := loadConfig(gs.FS, gs.Env)
	if cfgErr != nil && gs != nil {
		gs.Logger.Warnf("docs: ignoring invalid config: %v", cfgErr)
	}

	baseW := cmd.OutOrStdout()
	var buf *bytes.Buffer
	w := baseW

	if cfg.Renderer != "" && gs.Stdout.IsTTY {
		buf = &bytes.Buffer{}
		w = buf
	}

	depth := cfg.Depth
	if opts.depth > 0 {
		depth = opts.depth
	}
	env := &docsEnv{FS: gs.FS, CacheDir: cacheDir, Version: version, Depth: depth}
	if err := showDocs(env, w, idx, args); err != nil {
		return err
	}
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
