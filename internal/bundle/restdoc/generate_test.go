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
