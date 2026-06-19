package bundle_test

import (
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/grafana/xk6-docs/docs"
	"github.com/grafana/xk6-docs/internal/bundle"
)

// TestBuild_WithExtraSections verifies the injection hook: a generator passed
// via WithExtraSections has its sections merged into sections.json (and wired
// by populateChildren) alongside the sections walked from the k6-docs tree.
func TestBuild_WithExtraSections(t *testing.T) {
	t.Parallel()

	afs := bundle.NewOSFS()
	src := t.TempDir()
	out := t.TempDir()

	// Minimal k6-docs tree: a version root with one topic page.
	versionRoot := filepath.Join(src, "docs", "sources", "k6", "v0.99.x")
	if err := afs.MkdirAll(versionRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	topic := "---\ntitle: Topic\nweight: 10\n---\n\n# Topic\n"
	if err := afs.WriteFile(filepath.Join(versionRoot, "topic.md"), []byte(topic), 0o600); err != nil {
		t.Fatal(err)
	}

	// extraSections writes one page and returns its section.
	extra := func(afs docs.FS, markdownDir string) ([]docs.Section, error) {
		p := filepath.Join(markdownDir, "extra", "page.md")
		if err := afs.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			return nil, err
		}
		if err := afs.WriteFile(p, []byte("# Extra page\n"), 0o600); err != nil {
			return nil, err
		}
		return []docs.Section{{
			Slug: "extra/page", RelPath: "extra/page.md", Title: "Extra page", Category: "extra",
		}}, nil
	}

	if err := bundle.Build("v0.99.x", src, out, afs, io.Discard, bundle.WithExtraSections(extra)); err != nil {
		t.Fatalf("Build: %v", err)
	}

	// sections.json must contain both the walked topic and the injected page.
	data, err := afs.ReadFile(filepath.Join(out, "sections.json"))
	if err != nil {
		t.Fatalf("read sections.json: %v", err)
	}
	var idx docs.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("unmarshal sections.json: %v", err)
	}
	slugs := make(map[string]bool, len(idx.Sections))
	for _, s := range idx.Sections {
		slugs[s.Slug] = true
	}
	if !slugs["topic"] {
		t.Error("expected walked section \"topic\" in sections.json")
	}
	if !slugs["extra/page"] {
		t.Error("expected injected section \"extra/page\" in sections.json")
	}

	// The injected markdown file was written under the bundle.
	if _, err := afs.ReadFile(filepath.Join(out, "markdown", "extra", "page.md")); err != nil {
		t.Errorf("injected markdown not written: %v", err)
	}
}
