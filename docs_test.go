package docs

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

func newTestGlobalState(t *testing.T, afs fsext.Fs) *state.GlobalState {
	t.Helper()

	gs := state.NewGlobalState(context.Background())
	gs.FS = afs
	gs.Env = map[string]string{}

	return gs
}

// setupTestCache creates an in-memory directory with sections.json and markdown files,
// returning the filesystem and cache dir path.
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
			// k6-http-get's childName resolves to "get" (same as the existing get child)
			// because childName strips the parent prefix "k6-http-".
			// This triggers deduplication in printAlignedList.
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

	// Create markdown files. Content is raw-ish (shared shortcodes resolved,
	// but frontmatter and other shortcodes still present), matching how the
	// prepare command now generates cached content.
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

	// best_practices.md lives at the cache root (same as cmd/prepare output).
	bpPath := filepath.Join(dir, "best_practices.md")
	if err := fsext.WriteFile(afs, bpPath, []byte("---\ntitle: Best Practices\n---\nFollow these best practices for k6.\n"), 0o644); err != nil {
		t.Fatalf("write best_practices.md: %v", err)
	}

	// Reload from the in-memory FS so bySlug map is built (validates sections.json).
	if _, err = LoadIndex(afs, dir); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	return afs, dir
}

// setupTestdataCache copies testdata/cache/ from the real filesystem into an
// in-memory MemMapFs, returning the filesystem and cache dir path.
func setupTestdataCache(t *testing.T) (fsext.Fs, string) {
	t.Helper()

	afs := fsext.NewMemMapFs()
	cacheDir := "/tmp/testcache"

	srcDir := filepath.Join("testdata", "cache")
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcDir, path)
		target := filepath.Join(cacheDir, rel)
		if d.IsDir() {
			return afs.MkdirAll(target, 0o755)
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return fsext.WriteFile(afs, target, content, 0o644)
	})
	if err != nil {
		t.Fatalf("copy testdata: %v", err)
	}

	return afs, cacheDir
}

// setupCommand creates a test command environment and returns run/runErr helpers
// that execute the docs command with --cache-dir and --version pre-configured.
func setupCommand(t *testing.T) (func(*testing.T, ...string) string, func(*testing.T, ...string) error) {
	t.Helper()

	afs, cacheDir := setupTestdataCache(t)
	gs := newTestGlobalState(t, afs)

	run := func(t *testing.T, args ...string) string {
		t.Helper()
		cmd := newCmd(gs)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs(append([]string{"--cache-dir", cacheDir, "--version", "v0.55.x"}, args...))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute(%v): %v", args, err)
		}
		return buf.String()
	}

	runErr := func(t *testing.T, args ...string) error {
		t.Helper()
		cmd := newCmd(gs)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs(append([]string{"--cache-dir", cacheDir, "--version", "v0.55.x"}, args...))
		return cmd.Execute()
	}

	return run, runErr
}

var updateGolden = os.Getenv("UPDATE_GOLDEN") != "" //nolint:gochecknoglobals // test-only flag for golden file generation

func assertGolden(t *testing.T, name, actual string) {
	t.Helper()

	path := filepath.Join("testdata", "golden", name)
	if updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file %s: %v\nRun with UPDATE_GOLDEN=1 to create", path, err)
	}
	if actual != string(expected) {
		t.Errorf("output mismatch for %s:\n\nwant:\n%s\n\ngot:\n%s", name, string(expected), actual)
	}
}

func TestTOC(t *testing.T) {
	t.Parallel()

	run, _ := setupCommand(t)
	assertGolden(t, "toc.txt", run(t))
}

func TestViewTopic(t *testing.T) {
	t.Parallel()

	run, _ := setupCommand(t)

	t.Run("section_with_children", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http.txt", run(t, "http"))
	})
	t.Run("leaf_section", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http-get.txt", run(t, "http", "get"))
	})
	t.Run("parent_prefix_stripped", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http-cookiejar.txt", run(t, "http", "cookiejar"))
	})
	t.Run("deep_leaf", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http-cookiejar-clear.txt", run(t, "http", "cookiejar", "clear"))
	})
	t.Run("non_js_api_with_children", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/using-k6.txt", run(t, "using-k6"))
	})
}

func TestListTopics(t *testing.T) {
	t.Parallel()

	run, _ := setupCommand(t)

	t.Run("no_args", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "list/no-args.txt", run(t, "--list"))
	})
	t.Run("topic_with_children", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "list/http.txt", run(t, "--list", "http"))
	})
	t.Run("leaf", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "list/http-get.txt", run(t, "--list", "http", "get"))
	})
}

func TestAllDocs(t *testing.T) {
	t.Parallel()

	run, _ := setupCommand(t)
	assertGolden(t, "all.txt", run(t, "--all"))
}

func TestSearchCommand(t *testing.T) {
	t.Parallel()

	run, runErr := setupCommand(t)

	t.Run("by_title", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "search/scenarios.txt", run(t, "search", "Scenarios"))
	})
	t.Run("by_description", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "search/get-request.txt", run(t, "search", "GET request"))
	})
	t.Run("multi_group", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "search/k6.txt", run(t, "search", "k6"))
	})
	t.Run("body_content", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "search/websocket.txt", run(t, "search", "WebSocket example content"))
	})
	t.Run("no_results", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "search/no-results.txt", run(t, "search", "zzzznotfound"))
	})
	t.Run("title_match_before_body_match", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "search/http-module.txt", run(t, "search", "HTTP module"))
	})
	t.Run("missing_arg", func(t *testing.T) {
		t.Parallel()
		err := runErr(t, "search")
		if err == nil {
			t.Fatal("search with no args should error")
		}
	})
}

func TestBestPractices(t *testing.T) {
	t.Parallel()

	run, _ := setupCommand(t)

	t.Run("content", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "best-practices.txt", run(t, "best-practices"))
	})

	t.Run("missing_file", func(t *testing.T) {
		t.Parallel()

		noBPFs := fsext.NewMemMapFs()
		noBPDir := "/tmp/nobpcache"
		if err := noBPFs.MkdirAll(noBPDir, 0o755); err != nil {
			t.Fatal(err)
		}
		noBPIdx := &Index{Version: "v0.55.x", Sections: []Section{}}
		data, err := json.Marshal(noBPIdx)
		if err != nil {
			t.Fatal(err)
		}
		if err := fsext.WriteFile(noBPFs, filepath.Join(noBPDir, "sections.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}

		gs := newTestGlobalState(t, noBPFs)
		cmd := newCmd(gs)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--cache-dir", noBPDir, "--version", "v0.55.x", "best-practices"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error for missing best_practices.md")
		}
	})
}

func TestUnknownTopic(t *testing.T) {
	t.Parallel()

	_, runErr := setupCommand(t)
	err := runErr(t, "nonexistent-topic-xyz")
	if err == nil {
		t.Fatal("expected error")
	}
	assertGolden(t, "errors/unknown-topic.txt", err.Error())
}

func TestAutoDetectVersion(t *testing.T) {
	t.Parallel()

	// No --version flag, no K6_DOCS_VERSION env.
	// The command auto-detects k6 version from Go build info.
	// Since we import k6, DetectK6Version returns a real version.
	// We can't use testdata/cache (wrong version), so we just verify
	// the setup function returns a version and doesn't error.
	//
	// This tests the FULL precedence chain: flag -> env -> auto-detect.
	// With no flag and no env, auto-detect must succeed.

	afs, _ := setupTestdataCache(t)
	gs := newTestGlobalState(t, afs)
	// Don't set K6_DOCS_VERSION -- force auto-detect

	// We need a cache at the auto-detected version path.
	// DetectK6Version returns something like "v1.5.x".
	// Create a minimal cache at that path.
	version, err := DetectK6Version()
	if err != nil {
		// debug.ReadBuildInfo may not list go.k6.io/k6 as a dep in
		// test binaries depending on the Go version and build mode.
		t.Skipf("DetectK6Version unavailable in this build: %v", err)
	}

	// Copy testdata cache to the auto-detected version dir too.
	// (The actual version in sections.json won't match but LoadIndex doesn't check.)
	autoDir, err := CacheDir(gs.Env, version)
	if err != nil {
		// CacheDir needs HOME. Set it.
		gs.Env["HOME"] = "/fakehome"
		autoDir, err = CacheDir(gs.Env, version)
		if err != nil {
			t.Fatalf("CacheDir: %v", err)
		}
	}

	// Copy the testdata cache to the auto-detected version cache dir.
	srcDir := filepath.Join("testdata", "cache")
	if walkErr := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(srcDir, path)
		target := filepath.Join(autoDir, rel)
		if d.IsDir() {
			return afs.MkdirAll(target, 0o755)
		}
		content, _ := os.ReadFile(path)
		return fsext.WriteFile(afs, target, content, 0o644)
	}); walkErr != nil {
		t.Fatalf("copy testdata to auto-detect dir: %v", walkErr)
	}

	// Also need to set HOME so CacheDir works inside setup().
	if gs.Env["HOME"] == "" {
		gs.Env["HOME"] = "/fakehome"
	}

	// Run without --version and without --cache-dir flags
	cmd := newCmd(gs)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// No --version, no --cache-dir
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("auto-detect path failed: %v\nversion: %s", err, version)
	}

	// Should show the auto-detected version in the header
	out := buf.String()
	if !strings.Contains(out, "k6 Documentation ("+version+")") {
		t.Errorf("expected auto-detected version %s in output, got:\n%s", version, out)
	}
}

func TestVersionPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("flag_overrides_env", func(t *testing.T) {
		t.Parallel()
		afs, cacheDir := setupTestdataCache(t)
		gs := newTestGlobalState(t, afs)
		gs.Env["K6_DOCS_VERSION"] = "v9.9.x"

		cmd := newCmd(gs)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--cache-dir", cacheDir, "--version", "v0.55.x"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute: %v", err)
		}
		if !strings.Contains(buf.String(), "k6 Documentation (v0.55.x)") {
			t.Error("--version flag should override K6_DOCS_VERSION env")
		}
	})

	t.Run("env_used_when_no_flag", func(t *testing.T) {
		t.Parallel()
		afs, cacheDir := setupTestdataCache(t)
		gs := newTestGlobalState(t, afs)
		gs.Env["K6_DOCS_VERSION"] = "v0.55.x"

		cmd := newCmd(gs)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--cache-dir", cacheDir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute: %v", err)
		}
		if !strings.Contains(buf.String(), "k6 Documentation (v0.55.x)") {
			t.Error("K6_DOCS_VERSION env should be used when no --version flag")
		}
	})

	t.Run("cache_dir_env_used_when_no_flag", func(t *testing.T) {
		t.Parallel()
		afs, cacheDir := setupTestdataCache(t)
		gs := newTestGlobalState(t, afs)
		gs.Env["K6_DOCS_CACHE_DIR"] = cacheDir

		cmd := newCmd(gs)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--version", "v0.55.x"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute: %v", err)
		}
		if !strings.Contains(buf.String(), "k6 Documentation (v0.55.x)") {
			t.Error("K6_DOCS_CACHE_DIR env should be used when no --cache-dir flag")
		}
	})

	t.Run("cache_dir_flag_overrides_env", func(t *testing.T) {
		t.Parallel()
		afs, cacheDir := setupTestdataCache(t)
		gs := newTestGlobalState(t, afs)
		gs.Env["K6_DOCS_CACHE_DIR"] = "/nonexistent/path"

		cmd := newCmd(gs)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--cache-dir", cacheDir, "--version", "v0.55.x"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute: %v", err)
		}
		if !strings.Contains(buf.String(), "k6 Documentation (v0.55.x)") {
			t.Error("--cache-dir flag should override K6_DOCS_CACHE_DIR env")
		}
	})
}
