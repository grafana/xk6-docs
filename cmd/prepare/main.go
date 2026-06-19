// Command prepare processes the k6-docs repository into a doc bundle
// suitable for embedding. It walks the documentation tree, transforms
// Hugo shortcodes into clean markdown, and produces:
//   - markdown/ — transformed .md files (including best_practices.md)
//   - sections.json — structured index of all sections
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	git "github.com/go-git/go-git/v5"
	docs "github.com/grafana/xk6-docs/docs"
	"github.com/grafana/xk6-docs/internal/bundle"
	"github.com/grafana/xk6-docs/internal/bundle/restdoc"
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
	if err := run(k6Version, k6DocsPath, outputDir, http.DefaultClient, afs, log.Writer()); err != nil {
		log.Fatal(err)
	}
}

func run(
	k6Version, k6DocsPath, outputDir string,
	httpClient *http.Client, afs docs.FS, stderr io.Writer,
) error {
	// Ensure we have the k6-docs repo (cloned to a temp dir if no path given).
	docsPath, cleanup, err := ensureDocsRepo(k6DocsPath, defaultRepoURL, afs, stderr)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Fetch the latest v6 OpenAPI spec at build time, validating it before use.
	// Fall back to the spec embedded in restdoc on any problem so a build never
	// fails or ships an empty section because of a transient or bad response.
	restSections := func(fsys docs.FS, markdownDir string) ([]docs.Section, error) {
		return restdoc.Generate(fsys, markdownDir, fetchV6Override(httpClient, stderr))
	}

	if err := bundle.Build(k6Version, docsPath, outputDir, afs, stderr,
		bundle.WithExtraSections(restSections)); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(stderr, "Done: sections written")
	return nil
}

const defaultRepoURL = "https://github.com/grafana/k6-docs.git"

// defaultV6SpecURL is the live Grafana Cloud k6 v6 OpenAPI document, fetched
// at build time so each bundle bakes a current snapshot.
const defaultV6SpecURL = "https://api.k6.io/cloud/v6/openapi"

// fetchV6SpecTimeout bounds the build-time fetch so a slow or unreachable
// endpoint falls back to the embedded spec quickly.
const fetchV6SpecTimeout = 10 * time.Second

// fetchV6Spec downloads the live v6 OpenAPI document from defaultV6SpecURL
// using the given client. The bytes may be JSON or YAML; restdoc parses
// either. Returns an error on any transport failure or non-200 response so
// the caller can fall back to the embedded spec.
func fetchV6Spec(client *http.Client) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchV6SpecTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, defaultV6SpecURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req) //nolint:gosec // build-time GET of a fixed, trusted URL
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// fetchV6Override returns a freshly fetched v6 OpenAPI spec for
// restdoc.Generate, or nil to use the embedded copy. It fetches, then requires
// the bytes to parse and contain at least one operation; any failure
// (transport, non-200, parse error, or empty spec) logs a warning and returns
// nil so the build falls back to the embedded spec rather than failing or
// shipping an empty section.
func fetchV6Override(client *http.Client, stderr io.Writer) []byte {
	body, err := fetchV6Spec(client)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "warning: fetch v6 spec: %v; using embedded copy\n", err)
		return nil
	}
	spec, err := restdoc.LoadSpecFromBytes(body)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "warning: fetched v6 spec did not parse: %v; using embedded copy\n", err)
		return nil
	}
	if len(spec.Operations) == 0 {
		_, _ = fmt.Fprintln(stderr, "warning: fetched v6 spec had no operations; using embedded copy")
		return nil
	}
	return body
}

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
