// Package cli implements the k6 docs terminal UI.
package cli

import (
	"context"
	"io"
	"io/fs"
)

// Runtime holds the environment needed by the docs CLI.
// It is constructed once by the k6 adapter and passed to NewCommand.
type Runtime struct {
	Env    map[string]string
	Stdout io.Writer
	Stderr io.Writer
	IsTTY  bool
	OutFd  int // file descriptor for terminal width detection
	// NoColor and AutoExtRes are functions because k6 parses CLI flags
	// after the command tree is built — the values aren't available yet.
	NoColor    func() bool
	AutoExtRes func() bool
	BinaryPath string
	Ctx        context.Context
	Logf       func(string, ...any)
	FS         FS
}

// FS abstracts the filesystem operations the CLI needs.
type FS interface {
	Stat(name string) (fs.FileInfo, error)
	MkdirAll(path string, perm fs.FileMode) error
	RemoveAll(path string) error
	WriteFile(name string, data []byte, perm fs.FileMode) error
}
