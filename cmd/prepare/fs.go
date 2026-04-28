package main

import (
	"io/fs"
	"os"

	docs "github.com/grafana/xk6-docs/docs"
)

// newOsFS returns a docs.FS backed by the operating system.
func newOsFS() docs.FS { return osFS{} }

// osFS is the filesystem boundary. All os calls are confined here.
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
