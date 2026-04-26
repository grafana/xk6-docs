package docs

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
)

func newCmd(gs *state.GlobalState) *cobra.Command {
	return newDocsCmd(gs)
}

func newDocsCmd(gs *state.GlobalState) *cobra.Command {
	var opts docsOpts

	// agentHandled is set by PersistentPreRunE when non-TTY output
	// has been printed. RunE checks it and returns nil immediately.
	var agentHandled bool

	cmd := &cobra.Command{
		Use:   "docs [topic] [subtopic...]",
		Short: "Print k6 documentation",
		Long: `Offline k6 documentation in the terminal.

Auto-downloads docs matching your k6 version on first run, then serves
from cache. Topics resolve from space-separated args (e.g. "http get").
Use search to find topics quickly.`,

		Args: cobra.ArbitraryArgs,
		// Non-TTY: print agent guide and stop. Runs before any subcommand
		// (except skill, which overrides PersistentPreRunE).
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if gs.Stdout.IsTTY || opts.pager {
				return nil
			}
			env, err := setup(cmd.Context(), gs, opts.version, opts.cacheDir)
			if err != nil {
				return err
			}
			printAgentGuide(cmd.OutOrStdout(), agentCacheDir(env))
			agentHandled = true
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentHandled {
				return nil
			}
			return runDocs(gs, cmd, args, &opts)
		},
	}

	completionTopics := newTopicCompletion(gs, &opts)
	cmd.ValidArgsFunction = completionTopics

	cmd.PersistentFlags().StringVar(&opts.version, "version", "", "Override k6 version for docs lookup")
	cmd.PersistentFlags().StringVar(&opts.cacheDir, "cache-dir", "", "Override cache directory")
	cmd.PersistentFlags().IntVar(&opts.depth, "depth", 0, "Override subtopic depth (default 1)")
	cmd.PersistentFlags().BoolVarP(&opts.pager, "pager", "p", false, "Display with pager")
	cmd.PersistentFlags().IntVarP(&opts.width, "width", "w", 0, "Word-wrap width (0 for terminal width)")

	_ = cmd.RegisterFlagCompletionFunc("version", cobra.NoFileCompletions)
	_ = cmd.RegisterFlagCompletionFunc("cache-dir", completionDirs)
	_ = cmd.RegisterFlagCompletionFunc("depth", cobra.NoFileCompletions)

	cmd.AddCommand(newSearchCmd(gs, &opts, &agentHandled, completionTopics))
	cmd.AddCommand(newSkillCmd(gs))

	// --help bypasses PreRunE, so override it for non-TTY too.
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if !gs.Stdout.IsTTY {
			if env, err := setup(c.Context(), gs, opts.version, opts.cacheDir); err == nil {
				printAgentGuide(c.OutOrStdout(), agentCacheDir(env))
				return
			}
		}
		p := c.CommandPath()
		if c.Example == "" {
			c.Example = fmt.Sprintf(
				"  %s                        Show table of contents\n"+
					"  %s http                   Read the HTTP module docs\n"+
					"  %s http get               Read a specific topic\n"+
					"  %s search websocket       Search across all docs\n"+
					"  %s best-practices         Show best practices guide",
				p, p, p, p, p)
		}
		defaultHelp(c, args)
	})

	return cmd
}

// agentCacheDir returns the markdown directory path for the agent guide.
func agentCacheDir(env *docsEnv) string {
	return env.cacheDir
}

func newSearchCmd(
	gs *state.GlobalState, opts *docsOpts, agentHandled *bool, completionTopics completionFunc,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search documentation",
		Long:  "Fuzzy search across all topics (case-insensitive, ignores punctuation).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if *agentHandled {
				return nil
			}
			return runSearch(gs, cmd, args, opts)
		},
	}
	cmd.Hidden = true // completions adds it manually so we control when it appears
	cmd.ValidArgsFunction = completionTopics
	return cmd
}

func newSkillCmd(gs *state.GlobalState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill [directory]",
		Short: "Install the agent skill for AI coding tools",
		Long: `Install the k6 docs agent skill into a directory.

Without arguments, shows a table of supported agents and their skill directories.
With a directory argument, installs the skill files there.`,

		Args:              cobra.MaximumNArgs(1),
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // skip agent mode
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkill(gs.FS, cmd.OutOrStdout(), gs.Stdout.IsTTY, gs.CmdArgs[0], args)
		},
	}
	cmd.ValidArgsFunction = completionDirs
	return cmd
}

type docsOpts struct {
	version  string
	cacheDir string
	depth    int
	pager    bool
	width    int
}

// completionFunc is the signature for cobra completion functions.
type completionFunc = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

func runSearch(gs *state.GlobalState, cmd *cobra.Command, args []string, opts *docsOpts) error {
	rc, err := prepareRun(gs, cmd, opts)
	if err != nil {
		return err
	}
	printSearch(cmd.Context(), rc.env, rc.w, rc.env.idx, args)
	return rc.flush()
}

func runDocs(gs *state.GlobalState, cmd *cobra.Command, args []string, opts *docsOpts) error {
	rc, err := prepareRun(gs, cmd, opts)
	if err != nil {
		return err
	}
	if err := showDocs(cmd.Context(), rc.env, rc.w, rc.env.idx, args); err != nil {
		return err
	}
	return rc.flush()
}
