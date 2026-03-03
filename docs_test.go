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

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"k6-docs": func() {
			gs := state.NewGlobalState(context.Background())
			gs.Logger.SetLevel(logrus.DebugLevel)
			cmd := newCmd(gs)
			cmd.SetArgs(os.Args[1:])
			if err := cmd.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir: "testdata/scripts",
		Setup: func(env *testscript.Env) error {
			return copyDir("testdata/cache", filepath.Join(env.WorkDir, "cache"))
		},
		UpdateScripts: os.Getenv("UPDATE_GOLDEN") != "",
	})
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

// newTestGlobalState creates a GlobalState with an in-memory filesystem for unit tests.
// Used by TTY-dependent tests in config_test.go that can't be tested via testscript.
func newTestGlobalState(t *testing.T, afs fsext.Fs) *state.GlobalState {
	t.Helper()

	gs := state.NewGlobalState(context.Background())
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
			Children:    []string{"javascript-api/k6-http", "javascript-api/jslib"},
			IsIndex:     true,
		},
		{
			Slug:        "javascript-api/k6-http",
			RelPath:     "javascript-api/k6-http/_index.md",
			Title:       "k6/http",
			Description: "HTTP module for k6.",
			Weight:      1,
			Category:    "javascript-api",
			Children:    []string{"javascript-api/k6-http/get", "javascript-api/k6-http/post", "javascript-api/k6-http/cookiejar", "javascript-api/k6-http/k6-http-get"},
			IsIndex:     true,
		},
		{
			Slug:        "javascript-api/k6-http/get",
			RelPath:     "javascript-api/k6-http/get.md",
			Title:       "get",
			Description: "Make an HTTP GET request.",
			Weight:      1,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "javascript-api/k6-http/post",
			RelPath:     "javascript-api/k6-http/post.md",
			Title:       "post",
			Description: "Make an HTTP POST request.",
			Weight:      2,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "javascript-api/k6-http/k6-http-get",
			RelPath:     "javascript-api/k6-http/k6-http-get.md",
			Title:       "get (alternate)",
			Description: "Alternate GET endpoint.",
			Weight:      4,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "javascript-api/k6-http/cookiejar",
			RelPath:     "javascript-api/k6-http/cookiejar/_index.md",
			Title:       "CookieJar",
			Description: "HTTP cookie jar.",
			Weight:      3,
			Category:    "javascript-api",
			Children:    []string{"javascript-api/k6-http/cookiejar/cookiejar-clear"},
			IsIndex:     true,
		},
		{
			Slug:        "javascript-api/k6-http/cookiejar/cookiejar-clear",
			RelPath:     "javascript-api/k6-http/cookiejar/cookiejar-clear.md",
			Title:       "CookieJar.clear",
			Description: "Clear all cookies.",
			Weight:      1,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "javascript-api/jslib",
			RelPath:     "javascript-api/jslib/_index.md",
			Title:       "jslib",
			Description: "JavaScript utility library.",
			Weight:      5,
			Category:    "javascript-api",
			Children:    nil,
			IsIndex:     true,
		},
		{
			Slug:        "using-k6",
			RelPath:     "using-k6/_index.md",
			Title:       "Using k6",
			Description: "Learn how to use k6.",
			Weight:      2,
			Category:    "using-k6",
			Children:    []string{"using-k6/scenarios"},
			IsIndex:     true,
		},
		{
			Slug:        "using-k6/scenarios",
			RelPath:     "using-k6/scenarios.md",
			Title:       "Scenarios",
			Description: "Configure test scenarios.",
			Weight:      1,
			Category:    "using-k6",
			Children:    nil,
			IsIndex:     false,
		},
		{
			Slug:        "examples",
			RelPath:     "examples/_index.md",
			Title:       "Examples",
			Description: "Example k6 scripts.",
			Weight:      3,
			Category:    "examples",
			Children:    []string{"examples/websockets"},
			IsIndex:     true,
		},
		{
			Slug:        "examples/websockets",
			RelPath:     "examples/websockets.md",
			Title:       "WebSockets",
			Description: "WebSocket load testing examples including real-time bidirectional communication patterns and analysis",
			Weight:      1,
			Category:    "examples",
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
		"javascript-api/_index.md":                            "---\ntitle: 'JavaScript API'\n---\n# JavaScript API\n\nThe JavaScript API reference.\n",
		"javascript-api/k6-http/_index.md":                    "---\ntitle: 'k6/http'\n---\n# k6/http\n\nThe HTTP module.\n",
		"javascript-api/jslib/_index.md":                      "---\ntitle: 'jslib'\n---\n# jslib\n\nJavaScript utility library reference.\n",
		"javascript-api/k6-http/get.md":                       "---\ntitle: 'get'\n---\n## http.get(url)\n\nMake a GET request.\n",
		"javascript-api/k6-http/post.md":                      "---\ntitle: 'post'\n---\n## http.post(url, body)\n\nMake a POST request.\n",
		"javascript-api/k6-http/k6-http-get.md":               "---\ntitle: 'get (alternate)'\n---\n## http.get(url) [alternate]\n\nAlternate GET endpoint.\n",
		"javascript-api/k6-http/cookiejar/_index.md":          "---\ntitle: 'CookieJar'\n---\n# CookieJar\n\nHTTP cookie jar reference.\n",
		"javascript-api/k6-http/cookiejar/cookiejar-clear.md": "---\ntitle: 'CookieJar.clear'\n---\n## CookieJar.clear()\n\nClears all cookies.\n",
		"using-k6/_index.md":                                  "---\ntitle: 'Using k6'\n---\n# Using k6\n\nGuide to using k6.\n",
		"using-k6/scenarios.md":                               "---\ntitle: 'Scenarios'\n---\n# Scenarios\n\nScenarios let you configure execution.\n",
		"examples/_index.md":                                  "---\ntitle: 'Examples'\n---\n# Examples\n\nExample scripts.\n",
		"examples/websockets.md":                              "---\ntitle: 'WebSockets'\n---\n# WebSockets\n\nWebSocket example content.\n",
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
