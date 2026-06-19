package restdoc

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grafana/xk6-docs/docs"
)

// memFS is a minimal in-memory docs.FS used to capture the files Generate
// writes without touching disk. Only WriteFile/MkdirAll are exercised by
// Generate; the rest satisfy the interface.
type memFS struct{ files map[string][]byte }

func newMemFS() *memFS { return &memFS{files: map[string][]byte{}} }

func (m *memFS) WriteFile(name string, data []byte, _ fs.FileMode) error {
	m.files[name] = append([]byte(nil), data...)
	return nil
}
func (m *memFS) MkdirAll(string, fs.FileMode) error { return nil }
func (m *memFS) RemoveAll(string) error             { return nil }
func (m *memFS) Open(string) (fs.File, error)       { return nil, fs.ErrNotExist }
func (m *memFS) ReadFile(name string) ([]byte, error) {
	if b, ok := m.files[name]; ok {
		return b, nil
	}
	return nil, fs.ErrNotExist
}
func (m *memFS) Stat(string) (fs.FileInfo, error)      { return nil, fs.ErrNotExist }
func (m *memFS) ReadDir(string) ([]fs.DirEntry, error) { return nil, fs.ErrNotExist }

func TestGenerate(t *testing.T) {
	t.Parallel()

	mfs := newMemFS()
	sections, err := Generate(mfs, "out/markdown", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	bySlug := make(map[string]docs.Section, len(sections))
	for _, s := range sections {
		bySlug[s.Slug] = s
	}

	// The three index pages exist and are marked as index sections.
	for _, want := range []string{"cloud-rest-api", "cloud-rest-api/v5", "cloud-rest-api/v6"} {
		s, ok := bySlug[want]
		if !ok {
			t.Fatalf("missing section %q", want)
		}
		if !s.IsIndex {
			t.Errorf("section %q: IsIndex = false, want true", want)
		}
		key := filepath.Join("out/markdown", filepath.FromSlash(s.RelPath))
		if _, ok := mfs.files[key]; !ok {
			t.Errorf("no index markdown written at %q", key)
		}
	}

	// Endpoint leaves exist for both versions, each with a markdown file
	// containing the rendered Invocation section.
	var v5, v6 int
	for _, s := range sections {
		if s.IsIndex {
			continue
		}
		key := filepath.Join("out/markdown", filepath.FromSlash(s.RelPath))
		body, ok := mfs.files[key]
		if !ok {
			t.Errorf("no markdown written for %q", s.Slug)
			continue
		}
		if !strings.Contains(string(body), "## Invocation") {
			t.Errorf("rendered page %q missing Invocation section", s.Slug)
		}
		switch {
		case strings.HasPrefix(s.Slug, "cloud-rest-api/v5/"):
			v5++
		case strings.HasPrefix(s.Slug, "cloud-rest-api/v6/"):
			v6++
		}
	}
	if v5 == 0 {
		t.Error("no v5 endpoint leaves generated")
	}
	if v6 == 0 {
		t.Error("no v6 endpoint leaves generated")
	}
}

// TestGenerate_V6Override verifies a supplied v6 spec replaces the embedded
// one. renderSpec (from render_test.go) defines operationId things_get.
func TestGenerate_V6Override(t *testing.T) {
	t.Parallel()

	mfs := newMemFS()
	sections, err := Generate(mfs, "out/markdown", []byte(renderSpec))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	found := false
	for _, s := range sections {
		if s.Slug == "cloud-rest-api/v6/things_get" {
			found = true
			break
		}
	}
	if !found {
		t.Error("v6 override not applied: expected slug cloud-rest-api/v6/things_get")
	}
}

// TestGenerate_RejectsUnsafeOperationID ensures operationIds that would escape
// the output directory (path traversal) or are empty are skipped rather than
// written, while well-formed operations still generate.
func TestGenerate_RejectsUnsafeOperationID(t *testing.T) {
	t.Parallel()

	const spec = `
openapi: 3.0.0
info:
  title: Unsafe
  version: 1.0.0
servers:
  - url: https://api.k6.io
paths:
  /good:
    get:
      operationId: good_op
      responses:
        "200":
          description: OK
  /evil:
    get:
      operationId: ../../../../etc/evil
      responses:
        "200":
          description: OK
  /empty:
    get:
      responses:
        "200":
          description: OK
`

	mfs := newMemFS()
	sections, err := Generate(mfs, "out/markdown", []byte(spec))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	good := false
	for _, s := range sections {
		if s.Slug == "cloud-rest-api/v6/good_op" {
			good = true
		}
		if strings.Contains(s.Slug, "evil") {
			t.Errorf("unsafe operationId produced a section: %q", s.Slug)
		}
	}
	if !good {
		t.Error("expected cloud-rest-api/v6/good_op section to be generated")
	}

	// No file is written outside the markdown directory.
	base := filepath.Clean("out/markdown")
	for key := range mfs.files {
		rel, err := filepath.Rel(base, key)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Errorf("file written outside markdown dir: %q", key)
		}
	}

	// The empty operationId must not collide with the v6 index page.
	if _, ok := mfs.files[filepath.Join("out/markdown", "cloud-rest-api", "v6.md")]; ok {
		t.Error("empty operationId wrote cloud-rest-api/v6.md (collides with v6 index)")
	}
}
