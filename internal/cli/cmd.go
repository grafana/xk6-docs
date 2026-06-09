package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// docsOpts holds CLI flags for the docs command.
type docsOpts struct {
	version  string
	source   string
	cacheDir string
	depth    int
	pager    bool
	width    int
}

// completionFunc is the signature for cobra completion functions.
type completionFunc = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

const defaultDepth = 1

// NewCommand builds the docs subcommand tree.
// It captures rt and dispatches to inner functions with specific values.
func NewCommand(rt *Runtime) *cobra.Command {
	var opts docsOpts
	var agentHandled bool

	cmd := buildDocsCmd(rt, &opts, &agentHandled)
	completionTopics := buildCompletionTopics(rt, &opts)
	cmd.ValidArgsFunction = completionTopics

	registerFlags(cmd, &opts)
	cmd.AddCommand(buildSearchCmd(rt, &opts, &agentHandled, completionTopics))
	cmd.AddCommand(buildSkillCmd(rt))
	setHelpFunc(cmd, rt, &opts)

	return cmd
}

func buildDocsCmd(rt *Runtime, opts *docsOpts, agentHandled *bool) *cobra.Command {
	withDocs := newWithDocs(rt, opts)

	return &cobra.Command{
		Use:   "docs [topic] [subtopic...]",
		Short: "Print k6 documentation",
		Long: `Offline k6 documentation in the terminal.

Auto-downloads docs matching your k6 version on first run, then serves
from cache. Topics resolve from space-separated args (e.g. "http get").
Use search to find topics quickly.

Use --source <k6-docs-path> to preview docs from a local k6-docs checkout
instead of downloading. With --source, --version defaults to "next" (the
in-development docs); pass --version to target another version directory
under docs/sources/k6/.`,
		Args: cobra.ArbitraryArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if rt.IsTTY || opts.pager {
				return nil
			}
			env, err := setup(cmd.Context(), rt.Env, rt.Logf, rt.FS, opts.version, opts.source, opts.cacheDir)
			if err != nil {
				return err
			}
			printAgentGuide(cmd.OutOrStdout(), env.cacheDir)
			*agentHandled = true
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if *agentHandled {
				return nil
			}
			return withDocs(cmd, func(env *docsEnv, w io.Writer) error {
				return showDocs(cmd.Context(), env, w, env.idx, args)
			})
		},
	}
}

func buildSearchCmd(
	rt *Runtime, opts *docsOpts, agentHandled *bool, completionTopics completionFunc,
) *cobra.Command {
	withDocs := newWithDocs(rt, opts)

	cmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search documentation",
		Long:  "Fuzzy search across all topics (case-insensitive, ignores punctuation).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if *agentHandled {
				return nil
			}
			return withDocs(cmd, func(env *docsEnv, w io.Writer) error {
				printSearch(cmd.Context(), env, w, env.idx, args)
				return nil
			})
		},
	}
	cmd.Hidden = true
	cmd.ValidArgsFunction = completionTopics
	return cmd
}

func buildSkillCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill [directory]",
		Short: "Install the agent skill for AI coding tools",
		Long: `Install the k6 docs agent skill into a directory.

Without arguments, shows a table of supported agents and their skill directories.
With a directory argument, installs the skill files there.`,
		Args:              cobra.MaximumNArgs(1),
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkill(rt.FS, cmd.OutOrStdout(), rt.IsTTY, rt.BinaryPath, args)
		},
	}
	cmd.ValidArgsFunction = completionDirs
	return cmd
}

func buildCompletionTopics(rt *Runtime, opts *docsOpts) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		idx, version := setupForCompletion(rt.Ctx, rt.Env, opts)
		if idx == nil && version != "" && rt.AutoExtRes() {
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

func registerFlags(cmd *cobra.Command, opts *docsOpts) {
	cmd.PersistentFlags().StringVar(&opts.version, "version", "", "Override k6 version for docs lookup")
	cmd.PersistentFlags().StringVar(&opts.source, "source", "", "Build docs from a local k6-docs checkout")
	cmd.PersistentFlags().StringVar(&opts.cacheDir, "cache-dir", "", "Override cache directory")
	cmd.PersistentFlags().IntVar(&opts.depth, "depth", 0, "Override subtopic depth (default 1)")
	cmd.PersistentFlags().BoolVarP(&opts.pager, "pager", "p", false, "Display with pager")
	cmd.PersistentFlags().IntVarP(&opts.width, "width", "w", 0, "Word-wrap width (0 for terminal width)")

	_ = cmd.RegisterFlagCompletionFunc("version", cobra.NoFileCompletions)
	_ = cmd.RegisterFlagCompletionFunc("source", completionDirs)
	_ = cmd.RegisterFlagCompletionFunc("cache-dir", completionDirs)
	_ = cmd.RegisterFlagCompletionFunc("depth", cobra.NoFileCompletions)
}

func setHelpFunc(cmd *cobra.Command, rt *Runtime, opts *docsOpts) {
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if !rt.IsTTY {
			if env, err := setup(c.Context(), rt.Env, rt.Logf, rt.FS, opts.version, opts.source, opts.cacheDir); err == nil {
				printAgentGuide(c.OutOrStdout(), env.cacheDir)
				return
			}
		}
		setHelpExample(c)
		defaultHelp(c, args)
	})
}

func setHelpExample(c *cobra.Command) {
	if c.Example != "" {
		return
	}
	p := c.CommandPath()
	c.Example = fmt.Sprintf(
		"  %s                        Show table of contents\n"+
			"  %s http                   Read the HTTP module docs\n"+
			"  %s http get               Read a specific topic\n"+
			"  %s search websocket       Search across all docs\n"+
			"  %s best-practices         Show best practices guide",
		p, p, p, p, p)
}

// newWithDocs returns a closure that handles setup, buffering, and flush
// shared between runDocs and runSearch RunE handlers.
func newWithDocs(
	rt *Runtime, opts *docsOpts,
) func(cmd *cobra.Command, fn func(*docsEnv, io.Writer) error) error {
	return func(cmd *cobra.Command, fn func(*docsEnv, io.Writer) error) error {
		env, err := setup(cmd.Context(), rt.Env, rt.Logf, rt.FS, opts.version, opts.source, opts.cacheDir)
		if err != nil {
			return err
		}
		env.depth = defaultDepth
		if opts.depth > 0 {
			env.depth = opts.depth
		}

		width := resolveWidth(opts.width, rt.OutFd)

		w := cmd.OutOrStdout()
		var buf *bytes.Buffer
		if rt.IsTTY && !rt.NoColor() || opts.pager {
			buf = &bytes.Buffer{}
			w = buf
		}

		if err := fn(env, w); err != nil {
			return err
		}

		if buf == nil || buf.Len() == 0 {
			return nil
		}
		if opts.pager {
			return flushPager(cmd.Context(), rt.Stdout, rt.Stderr, rt.Env["PAGER"], buf.String(), width)
		}
		return renderMarkdown(rt.Stdout, buf.String(), width)
	}
}

func resolveWidth(optWidth, outFd int) int {
	if optWidth > 0 {
		return optWidth
	}
	const defaultWidth = 80
	w, _, err := term.GetSize(outFd)
	if err == nil && w > 0 {
		return w
	}
	return defaultWidth
}

func flushPager(ctx context.Context, stdout, stderr io.Writer, pagerEnv, content string, width int) error {
	pagerCmd := pagerEnv
	if pagerCmd == "" {
		pagerCmd = "less -r"
	}
	parts := strings.Split(pagerCmd, " ")
	// pagerCmd is the user's $PAGER env var; intentionally user-controlled.
	c := exec.CommandContext(ctx, parts[0], parts[1:]...) // #nosec G204
	c.Stdout = stdout
	c.Stderr = stderr
	stdin, err := c.StdinPipe()
	if err != nil {
		return err
	}
	if err := c.Start(); err != nil {
		return err
	}
	err = renderMarkdown(stdin, content, width)
	_ = stdin.Close()
	if waitErr := c.Wait(); err == nil {
		err = waitErr
	}
	return err
}
