package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir: "testdata/scripts",
		Setup: func(env *testscript.Env) error {
			return copyDir("testdata/cache", filepath.Join(env.WorkDir, "cache"))
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"k6-docs": runK6DocsCmd,
		},
		UpdateScripts: os.Getenv("UPDATE_GOLDEN") != "",
	})
}

// runK6DocsCmd runs the docs command in-process for testscript, avoiding
// subprocess overhead. It injects the testscript's sandboxed environment
// into the GlobalState so env directives in txtar scripts work correctly.
func runK6DocsCmd(ts *testscript.TestScript, neg bool, args []string) {
	gs := state.NewGlobalState(context.Background())
	gs.Logger.SetLevel(logrus.DebugLevel)
	gs.Logger.SetOutput(ts.Stderr())
	gs.Env = map[string]string{
		"K6_DOCS_VERSION":   ts.Getenv("K6_DOCS_VERSION"),
		"K6_DOCS_CACHE_DIR": ts.Getenv("K6_DOCS_CACHE_DIR"),
		"HOME":              ts.Getenv("HOME"),
		"USERPROFILE":       ts.Getenv("USERPROFILE"),
	}

	cmd := newCmd(gs)
	cmd.SetOut(ts.Stdout())
	cmd.SetErr(ts.Stderr())
	cmd.SetArgs(args)

	err := cmd.Execute()
	if err != nil {
		_, _ = fmt.Fprintf(ts.Stderr(), "Error: %v\n", err)
	}
	if neg {
		if err == nil {
			ts.Fatalf("expected command to fail")
		}
	} else if err != nil {
		ts.Fatalf("unexpected command failure")
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

func TestPrintTree(t *testing.T) {
	t.Parallel()

	sections := []Section{
		{Slug: "alpha", Weight: 1, Category: "alpha", Children: []string{"alpha/child-1", "alpha/child-2"}},
		{Slug: "alpha/child-1", Weight: 1, Category: "alpha", Children: []string{"alpha/child-1/grandchild"}},
		{Slug: "alpha/child-1/grandchild", Weight: 1, Category: "alpha"},
		{Slug: "alpha/child-2", Weight: 2, Category: "alpha"},
		{Slug: "beta", Weight: 2, Category: "beta"},
	}
	idx := &Index{Sections: sections}
	idx.bySlug = make(map[string]*Section, len(sections))
	for i := range sections {
		idx.bySlug[sections[i].Slug] = &sections[i]
	}

	items := idx.TopLevel()

	t.Run("depth 1 prints flat list", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		printTree(&buf, idx, items, "", "", 1)
		want := "- alpha\n- beta\n"
		if buf.String() != want {
			t.Errorf("depth=1:\ngot:\n%s\nwant:\n%s", buf.String(), want)
		}
	})

	t.Run("depth 2 prints one level of children", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		printTree(&buf, idx, items, "", "", 2)
		want := "- alpha\n  - child-1\n  - child-2\n- beta\n"
		if buf.String() != want {
			t.Errorf("depth=2:\ngot:\n%s\nwant:\n%s", buf.String(), want)
		}
	})

	t.Run("depth 3 prints grandchildren", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		printTree(&buf, idx, items, "", "", 3)
		want := "- alpha\n  - child-1\n    - grandchild\n  - child-2\n- beta\n"
		if buf.String() != want {
			t.Errorf("depth=3:\ngot:\n%s\nwant:\n%s", buf.String(), want)
		}
	})

	t.Run("depth 0 prints nothing", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		printTree(&buf, idx, items, "", "", 0)
		if buf.String() != "" {
			t.Errorf("depth=0: got %q, want empty", buf.String())
		}
	})
}

func TestPrintSearchArgs(t *testing.T) {
	t.Parallel()

	afs, dir := setupTestCache(t)
	idx, err := LoadIndex(afs, dir)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	env := &docsEnv{FS: afs, CacheDir: dir, Version: "v0.55.x", Depth: 1}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "slash", args: []string{"k6-mod-b/leaf-one"}, want: "k6-mod-b"},
		{name: "space", args: []string{"k6-mod-b", "leaf-one"}, want: "k6-mod-b"},
		{name: "bare_slash", args: []string{"mod-b/leaf-one"}, want: "k6-mod-b"},
		{name: "full_slug", args: []string{"javascript-api", "k6-mod-b", "leaf-one"}, want: "k6-mod-b"},
		{name: "full_slug_bare", args: []string{"javascript-api", "mod-b", "leaf-one"}, want: "k6-mod-b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf strings.Builder
			printSearch(env, &buf, idx, tt.args)
			if !strings.Contains(buf.String(), tt.want) {
				t.Errorf("printSearch(%v) = %q, want substring %q", tt.args, buf.String(), tt.want)
			}
		})
	}
}

func TestPrintSearchNoResults(t *testing.T) {
	t.Parallel()

	afs, dir := setupTestCache(t)
	idx, err := LoadIndex(afs, dir)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	env := &docsEnv{FS: afs, CacheDir: dir, Version: "v0.55.x", Depth: 1}

	var buf strings.Builder
	printSearch(env, &buf, idx, []string{"zzzznotfound"})
	want := "(no results)\n"
	if buf.String() != want {
		t.Errorf("no results: got %q, want %q", buf.String(), want)
	}
}

func TestPrintSearchDepth(t *testing.T) {
	t.Parallel()

	afs, dir := setupTestCache(t)
	idx, err := LoadIndex(afs, dir)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	t.Run("depth 1 shows only groups", func(t *testing.T) {
		t.Parallel()
		env := &docsEnv{FS: afs, CacheDir: dir, Version: "v0.55.x", Depth: 1}
		var buf strings.Builder
		printSearch(env, &buf, idx, []string{"k6-mod-b"})
		got := buf.String()
		if !strings.Contains(got, "- k6-mod-b\n") {
			t.Errorf("depth 1: expected group header, got %q", got)
		}
		if strings.Contains(got, "  - ") {
			t.Errorf("depth 1: should not show children, got %q", got)
		}
	})

	t.Run("depth 2 shows groups and children", func(t *testing.T) {
		t.Parallel()
		env := &docsEnv{FS: afs, CacheDir: dir, Version: "v0.55.x", Depth: 2}
		var buf strings.Builder
		printSearch(env, &buf, idx, []string{"k6-mod-b"})
		got := buf.String()
		if !strings.Contains(got, "  - leaf-one") {
			t.Errorf("depth 2: expected children, got %q", got)
		}
	})
}

// newTestGlobalState creates a GlobalState with an in-memory filesystem for unit tests.
func newTestGlobalState(t *testing.T, afs fsext.Fs) *state.GlobalState {
	t.Helper()

	gs := state.NewGlobalState(t.Context())
	gs.FS = afs
	gs.Env = map[string]string{}

	return gs
}

// setupTestCache creates an in-memory cache with sections.json and markdown files.
// Used by TTY-dependent tests in config_test.go that can't be tested via testscript.
func setupTestCache(t *testing.T) (fsext.Fs, string) {
	t.Helper()

	afs := fsext.NewMemMapFs()
	dir := "/tmp/testcache"
	if err := afs.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	sections := []Section{
		{
			Slug:        "javascript-api",
			RelPath:     "javascript-api/_index.md",
			Title:       "JavaScript API",
			Description: "k6 JavaScript API reference.",
			Weight:      1,
			Category:    "javascript-api",
			Children:    []string{"javascript-api/k6-mod-a", "javascript-api/k6-mod-b", "javascript-api/lib-c"},
			IsIndex:     true,
		},
		{
			Slug:        "javascript-api/k6-mod-a",
			RelPath:     "javascript-api/k6-mod-a/_index.md",
			Title:       "k6/mod-a",
			Description: "Module A for k6.",
			Weight:      1,
			Category:    "javascript-api",
			Children:    []string{"javascript-api/k6-mod-a/fn-one", "javascript-api/k6-mod-a/fn-two", "javascript-api/k6-mod-a/child-a", "javascript-api/k6-mod-a/k6-mod-a-fn-one"},
			IsIndex:     true,
		},
		{
			Slug:        "javascript-api/k6-mod-a/fn-one",
			RelPath:     "javascript-api/k6-mod-a/fn-one.md",
			Title:       "fn-one",
			Description: "First function.",
			Weight:      1,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "javascript-api/k6-mod-a/fn-two",
			RelPath:     "javascript-api/k6-mod-a/fn-two.md",
			Title:       "fn-two",
			Description: "Second function.",
			Weight:      2,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "javascript-api/k6-mod-a/k6-mod-a-fn-one",
			RelPath:     "javascript-api/k6-mod-a/k6-mod-a-fn-one.md",
			Title:       "fn-one (alternate)",
			Description: "Alternate fn-one endpoint.",
			Weight:      4,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "javascript-api/k6-mod-a/child-a",
			RelPath:     "javascript-api/k6-mod-a/child-a/_index.md",
			Title:       "ChildA",
			Description: "Child A reference.",
			Weight:      3,
			Category:    "javascript-api",
			Children:    []string{"javascript-api/k6-mod-a/child-a/child-a-clear"},
			IsIndex:     true,
		},
		{
			Slug:        "javascript-api/k6-mod-a/child-a/child-a-clear",
			RelPath:     "javascript-api/k6-mod-a/child-a/child-a-clear.md",
			Title:       "ChildA.clear",
			Description: "Clear all items.",
			Weight:      1,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "javascript-api/lib-c",
			RelPath:     "javascript-api/lib-c/_index.md",
			Title:       "lib-c",
			Description: "Library C reference.",
			Weight:      5,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     true,
		},
		{
			Slug:        "javascript-api/k6-mod-b",
			RelPath:     "javascript-api/k6-mod-b/_index.md",
			Title:       "k6/mod-b",
			Description: "Module B for k6.",
			Weight:      4,
			Category:    "javascript-api",
			Children:    []string{"javascript-api/k6-mod-b/leaf-one"},
			IsIndex:     true,
		},
		{
			Slug:        "javascript-api/k6-mod-b/leaf-one",
			RelPath:     "javascript-api/k6-mod-b/leaf-one.md",
			Title:       "LeafOne",
			Description: "Represents a leaf node.",
			Weight:      1,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "alpha",
			RelPath:     "alpha/_index.md",
			Title:       "Alpha",
			Description: "Learn how to use k6.",
			Weight:      2,
			Category:    "alpha",
			Children:    []string{"alpha/topic-one"},
			IsIndex:     true,
		},
		{
			Slug:        "alpha/topic-one",
			RelPath:     "alpha/topic-one.md",
			Title:       "TopicOne",
			Description: "Configure test topic-one.",
			Weight:      1,
			Category:    "alpha",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "beta",
			RelPath:     "beta/_index.md",
			Title:       "Beta",
			Description: "Example k6 scripts.",
			Weight:      3,
			Category:    "beta",
			Children:    []string{"beta/topic-four"},
			IsIndex:     true,
		},
		{
			Slug:        "beta/topic-four",
			RelPath:     "beta/topic-four.md",
			Title:       "TopicFour",
			Description: "TopicFour load testing examples including real-time bidirectional communication patterns and analysis",
			Weight:      1,
			Category:    "beta",
			Children:    nil,
			IsIndex:     false,
		},
	}

	idx := &Index{
		Version:  "v0.55.x",
		Sections: sections,
	}

	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	if err := fsext.WriteFile(afs, filepath.Join(dir, "sections.json"), data, 0o644); err != nil {
		t.Fatalf("write sections.json: %v", err)
	}

	mdFiles := map[string]string{
		"javascript-api/_index.md":                         "---\ntitle: 'JavaScript API'\n---\n# JavaScript API\n\nThe JavaScript API reference.\n",
		"javascript-api/k6-mod-a/_index.md":                "---\ntitle: 'k6/mod-a'\n---\n# k6/mod-a\n\nThe mod-a module.\n",
		"javascript-api/lib-c/_index.md":                   "---\ntitle: 'lib-c'\n---\n# lib-c\n\nLibrary C reference.\n",
		"javascript-api/k6-mod-b/_index.md":                "---\ntitle: 'k6/mod-b'\n---\n# k6/mod-b\n\nModule B.\n",
		"javascript-api/k6-mod-b/leaf-one.md":              "---\ntitle: 'LeafOne'\n---\n# LeafOne\n\nRepresents a leaf node.\n",
		"javascript-api/k6-mod-a/fn-one.md":                "---\ntitle: 'fn-one'\n---\n## modA.fnOne(url)\n\nFirst function call.\n",
		"javascript-api/k6-mod-a/fn-two.md":                "---\ntitle: 'fn-two'\n---\n## modA.fnTwo(url, body)\n\nSecond function call.\n",
		"javascript-api/k6-mod-a/k6-mod-a-fn-one.md":       "---\ntitle: 'fn-one (alternate)'\n---\n## modA.fnOne(url) [alternate]\n\nAlternate fn-one endpoint.\n",
		"javascript-api/k6-mod-a/child-a/_index.md":        "---\ntitle: 'ChildA'\n---\n# ChildA\n\nChild A reference.\n",
		"javascript-api/k6-mod-a/child-a/child-a-clear.md": "---\ntitle: 'ChildA.clear'\n---\n## ChildA.clear()\n\nClears all items.\n",
		"alpha/_index.md":                                  "---\ntitle: 'Alpha'\n---\n# Alpha\n\nGuide to Alpha.\n",
		"alpha/topic-one.md":                               "---\ntitle: 'TopicOne'\n---\n# TopicOne\n\nTopicOne lets you configure execution.\n",
		"beta/_index.md":                                   "---\ntitle: 'Beta'\n---\n# Beta\n\nExample scripts.\n",
		"beta/topic-four.md":                               "---\ntitle: 'TopicFour'\n---\n# TopicFour\n\nTopicFour example content.\n",
	}

	for relPath, content := range mdFiles {
		fullPath := filepath.Join(dir, "markdown", relPath)
		if err := afs.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := fsext.WriteFile(afs, fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", fullPath, err)
		}
	}

	bpPath := filepath.Join(dir, "best_practices.md")
	if err := fsext.WriteFile(afs, bpPath, []byte("---\ntitle: Best Practices\n---\nFollow these best practices for k6.\n"), 0o644); err != nil {
		t.Fatalf("write best_practices.md: %v", err)
	}

	if _, err = LoadIndex(afs, dir); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	return afs, dir
}
