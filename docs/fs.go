package docs

import (
	"io/fs"
	"os"
)

// WriteFileFS writes files.
type WriteFileFS interface {
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// WriteDirFS manages directories.
type WriteDirFS interface {
	MkdirAll(path string, perm fs.FileMode) error
	RemoveAll(path string) error
}

// FS composes read and write filesystem operations.
type FS interface {
	fs.ReadFileFS
	fs.StatFS
	fs.ReadDirFS
	WriteFileFS
	WriteDirFS
}

// osFS is the filesystem abstraction boundary. All os package calls
// in the docs module are confined to this single type.
type osFS struct{}

func (osFS) Open(name string) (fs.File, error) {
	return os.Open(name) //nolint:forbidigo,gosec // FS boundary
}

func (osFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name) //nolint:forbidigo,gosec // FS boundary
}

func (osFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name) //nolint:forbidigo // FS boundary
}

func (osFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name) //nolint:forbidigo // FS boundary
}

func (osFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm) //nolint:forbidigo // FS boundary
}

func (osFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm) //nolint:forbidigo // FS boundary
}

func (osFS) RemoveAll(path string) error {
	return os.RemoveAll(path) //nolint:forbidigo // FS boundary
}
