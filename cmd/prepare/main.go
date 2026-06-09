// Command prepare processes the k6-docs repository into a doc bundle
// suitable for embedding. It walks the documentation tree, transforms
// Hugo shortcodes into clean markdown, and produces:
//   - markdown/ — transformed .md files (including best_practices.md)
//   - sections.json — structured index of all sections
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	docs "github.com/grafana/xk6-docs/docs"
	"github.com/grafana/xk6-docs/internal/bundle"
)

func main() {
	log.SetFlags(0)

	var (
		k6Version  string
		k6DocsPath string
		outputDir  string
	)

	flag.StringVar(&k6Version, "k6-version", "", "k6 docs version (e.g. v1.5.x) — required")
	flag.StringVar(&k6DocsPath, "k6-docs-path", "", "local path to k6-docs repo (cloned if empty)")
	flag.StringVar(&outputDir, "output-dir", "dist/", "output directory")
	flag.Parse()

	if k6Version == "" {
		log.Fatal("--k6-version is required")
	}

	afs := bundle.NewOSFS()
	if err := run(k6Version, k6DocsPath, outputDir, afs, log.Writer()); err != nil {
		log.Fatal(err)
	}
}

func run(
	k6Version, k6DocsPath, outputDir string,
	afs docs.FS, stderr io.Writer,
) error {
	// Ensure we have the k6-docs repo (cloned to a temp dir if no path given).
	docsPath, cleanup, err := ensureDocsRepo(k6DocsPath, defaultRepoURL, afs, stderr)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	if err := bundle.Build(k6Version, docsPath, outputDir, afs, stderr); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(stderr, "Done: sections written")
	return nil
}

const defaultRepoURL = "https://github.com/grafana/k6-docs.git"

// ensureDocsRepo returns the path to the k6-docs repo. If k6DocsPath is empty,
// it clones from repoURL into a temp directory and returns a cleanup function.
func ensureDocsRepo(
	k6DocsPath, repoURL string, afs docs.FS, stderr io.Writer,
) (string, func(), error) {
	if k6DocsPath != "" {
		return k6DocsPath, nil, nil
	}

	tmpDir, err := mkTempDir(afs)
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	_, _ = fmt.Fprintln(stderr, "Cloning k6-docs repository...")
	_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:      repoURL,
		Depth:    1,
		Progress: stderr,
	})
	if err != nil {
		_ = afs.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("clone k6-docs: %w", err)
	}

	cleanup := func() { _ = afs.RemoveAll(tmpDir) }
	return tmpDir, cleanup, nil
}

func mkTempDir(afs docs.FS) (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	dir := filepath.Join("/tmp", fmt.Sprintf("k6-docs-%x", buf))
	if err := afs.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}
