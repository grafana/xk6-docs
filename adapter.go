package docs

import (
	"io/fs"

	"github.com/grafana/xk6-docs/internal/cli"
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

func newCmd(gs *state.GlobalState) *cobra.Command {
	return cli.NewCommand(&cli.Runtime{
		Env:        gs.Env,
		Stdout:     gs.Stdout.Writer,
		Stderr:     gs.Stderr.Writer,
		IsTTY:      gs.Stdout.IsTTY,
		OutFd:      gs.Stdout.RawOutFd,
		NoColor:    func() bool { return gs.Flags.NoColor },
		AutoExtRes: func() bool { return gs.Flags.AutoExtensionResolution },
		BinaryPath: gs.CmdArgs[0],
		Ctx:        gs.Ctx,
		Logf:       gs.Logger.Infof,
		FS:         &docsFS{gs.FS},
	})
}

// docsFS adapts k6's fsext.Fs to the cli.FS interface.
type docsFS struct{ fsext.Fs }

func (f *docsFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return fsext.WriteFile(f.Fs, name, data, perm)
}
