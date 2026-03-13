package docs

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"go.k6.io/k6/lib/fsext"
)

func TestDownloadURL(t *testing.T) {
	t.Parallel()

	got := downloadURL("v1.0.0")
	want := "https://github.com/grafana/xk6-docs/releases/download/doc-bundles/docs-v1.0.0.tar.zst"
	if got != want {
		t.Errorf("downloadURL(v1.0.0) = %q, want %q", got, want)
	}
}

func TestCacheDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{"HOME set", map[string]string{"HOME": "/somepath"}, false},
		{"USERPROFILE fallback", map[string]string{"USERPROFILE": `C:\Users\me`}, false},
		{"neither set", map[string]string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir, err := CacheDir(tt.env, "v1.2.3")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("CacheDir: %v", err)
			}
			if !strings.HasSuffix(dir, filepath.Join("k6", "docs", "v1.2.3")) {
				t.Errorf("CacheDir = %q, want suffix %q", dir, filepath.Join("k6", "docs", "v1.2.3"))
			}
		})
	}
}

func TestIsCached(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}

	if IsCached(afs, env, "nonexistent-version-xyz") {
		t.Error("IsCached returned true for a version that should not exist")
	}

	dir, err := CacheDir(env, "test-cached-version")
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	if err := afs.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if !IsCached(afs, env, "test-cached-version") {
		t.Error("IsCached returned false after creating cache directory")
	}
}

func TestIsValidVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		valid   bool
	}{
		{"v1.6.x", true},
		{"v0.55.x", true},
		{"v1.5.0-rc.1", true},
		{"next", true},
		{"", false},
		{".", false},
		{"..", false},
		{"...", false},
		{"../escape", false},
		{"v1.6.x/../../etc", false},
		{"v1.6 x", false},
		{"v1.6\tx", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			t.Parallel()
			if got := isValidVersion(tt.version); got != tt.valid {
				t.Errorf("isValidVersion(%q) = %v, want %v", tt.version, got, tt.valid)
			}
		})
	}
}

func TestEnsureDocsRejectsTraversalVersion(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}

	for _, version := range []string{"..", ".", "../escape"} {
		_, err := EnsureDocs(context.Background(), afs, env, version, &http.Client{})
		if err == nil {
			t.Errorf("EnsureDocs(%q) should reject traversal version", version)
		}
	}
}

func TestIsCachedReturnsFalseOnCacheDirFailure(t *testing.T) {
	t.Parallel()

	if IsCached(fsext.NewMemMapFs(), map[string]string{}, "v1.0.0") {
		t.Error("IsCached should return false when CacheDir fails")
	}
}

func TestExtract(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	archive := buildTarZst(t, map[string]string{
		"readme.txt":        "hello world",
		"subdir/nested.txt": "nested content",
	})

	dest := "/tmp/extract-test"
	if err := afs.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := extract(afs, archive, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}

	assertFileContent(t, afs, filepath.Join(dest, "readme.txt"), "hello world")
	assertFileContent(t, afs, filepath.Join(dest, "subdir", "nested.txt"), "nested content")
}

func TestExtractRejectsBadPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{"traversal", "../escape.txt"},
		{"absolute", "/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			afs := fsext.NewMemMapFs()
			archive := buildTarZstRaw(t, []tarEntry{{name: tt.path, content: "evil"}})
			dest := "/tmp/" + tt.name + "-test"
			if err := afs.MkdirAll(dest, 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := extract(afs, archive, dest); err == nil {
				t.Fatal("extract should reject bad path")
			}
		})
	}
}

func TestExtractSkipsNonRegularEntries(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	archive := buildTarZstRaw(t, []tarEntry{
		{name: "regular.txt", content: "hello"},
		{name: "link.txt", typeflag: tar.TypeSymlink, content: ""},
	})

	dest := "/tmp/skip-nonreg-test"
	if err := afs.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := extract(afs, archive, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}

	assertFileContent(t, afs, filepath.Join(dest, "regular.txt"), "hello")
	if _, err := afs.Stat(filepath.Join(dest, "link.txt")); err == nil {
		t.Error("symlink entry should have been skipped")
	}
}

func TestEnsureDocs(t *testing.T) {
	t.Parallel()

	srv, _ := newFakeBundleServer(t, map[string]string{"doc.txt": "documentation content"})
	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}
	version := "test-ensure"

	dir, err := CacheDir(env, version)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}

	client := newFakeClient(srv)
	got, err := EnsureDocs(context.Background(), afs, env, version, client)
	if err != nil {
		t.Fatalf("EnsureDocs: %v", err)
	}
	if got != dir {
		t.Errorf("EnsureDocs returned %q, want %q", got, dir)
	}
	assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "documentation content")

	// Second call serves from cache — content unchanged.
	got2, err := EnsureDocs(context.Background(), afs, env, version, client)
	if err != nil {
		t.Fatalf("EnsureDocs second: %v", err)
	}
	if got2 != dir {
		t.Errorf("second EnsureDocs returned %q, want %q", got2, dir)
	}
}

func TestEnsureDocsRejectsOversizedFile(t *testing.T) {
	t.Parallel()

	srv := newLargeFileBundleServer(t, "big.bin", maxFileSize+1)
	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}

	_, err := EnsureDocs(context.Background(), afs, env, "test-oversize", newFakeClient(srv))
	if err == nil {
		t.Fatal("EnsureDocs should reject oversized file")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("expected maximum size error, got: %v", err)
	}
}

func TestEnsureDocsPermissions(t *testing.T) {
	t.Parallel()

	srv, _ := newFakeBundleServer(t, map[string]string{
		"topfile.txt":       "content",
		"subdir/nested.txt": "nested",
	})
	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}
	version := "test-perms"

	dir, err := CacheDir(env, version)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	if _, err := EnsureDocs(context.Background(), afs, env, version, newFakeClient(srv)); err != nil {
		t.Fatalf("EnsureDocs: %v", err)
	}

	dirInfo, err := afs.Stat(filepath.Join(dir, "subdir"))
	if err != nil {
		t.Fatalf("Stat(subdir): %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o750 {
		t.Errorf("directory permission = %04o, want 0750", got)
	}

	for _, name := range []string{"topfile.txt", filepath.Join("subdir", "nested.txt")} {
		info, err := afs.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("Stat(%s): %v", name, err)
		}
		if got := info.Mode().Perm(); got != 0o640 {
			t.Errorf("file %s permission = %04o, want 0640", name, got)
		}
	}
}

func TestExtractCleansUpOnFailure(t *testing.T) {
	t.Parallel()

	srv := newMixedBundleServer(t,
		[]tarEntry{{name: "valid.txt", content: "ok"}},
		"oversized.bin", maxFileSize+1,
	)
	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}
	version := "test-cleanup"

	dir, err := CacheDir(env, version)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}

	_, err = EnsureDocs(context.Background(), afs, env, version, newFakeClient(srv))
	if err == nil {
		t.Fatal("EnsureDocs should fail on oversized file")
	}
	if _, statErr := afs.Stat(dir); statErr == nil {
		t.Errorf("cache directory %q still exists after failed extraction", dir)
	}
}

func TestEnsureDocsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}

	_, err := EnsureDocs(context.Background(), afs, env, "test-httperr", newFakeClient(srv))
	if err == nil {
		t.Fatal("EnsureDocs should fail on HTTP 404")
	}
}

func TestMultiVersionCoexistence(t *testing.T) {
	t.Parallel()

	srvV1, _ := newFakeBundleServer(t, map[string]string{"doc.txt": "version one content"})
	srvV2, _ := newFakeBundleServer(t, map[string]string{"doc.txt": "version two content"})

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}

	dirV1, err := EnsureDocs(context.Background(), afs, env, "v1.0.x", newFakeClient(srvV1))
	if err != nil {
		t.Fatalf("EnsureDocs v1: %v", err)
	}
	dirV2, err := EnsureDocs(context.Background(), afs, env, "v2.0.x", newFakeClient(srvV2))
	if err != nil {
		t.Fatalf("EnsureDocs v2: %v", err)
	}

	if dirV1 == dirV2 {
		t.Fatal("different versions should have different cache dirs")
	}
	assertFileContent(t, afs, filepath.Join(dirV1, "doc.txt"), "version one content")
	assertFileContent(t, afs, filepath.Join(dirV2, "doc.txt"), "version two content")
	if !IsCached(afs, env, "v1.0.x") {
		t.Error("v1.0.x should be cached")
	}
	if !IsCached(afs, env, "v2.0.x") {
		t.Error("v2.0.x should be cached")
	}
}

func TestStaleness(t *testing.T) {
	t.Parallel()

	// download stores etag and last_check — only one EnsureDocs call needed.
	t.Run("download stores etag and last_check", func(t *testing.T) {
		t.Parallel()

		srv, _ := newFakeBundleServer(t, map[string]string{"doc.txt": "content"})
		afs := fsext.NewMemMapFs()
		env := map[string]string{"HOME": "/fakehome"}
		version := "test-staleness-store"

		dir, err := CacheDir(env, version)
		if err != nil {
			t.Fatalf("CacheDir: %v", err)
		}
		if _, err := EnsureDocs(context.Background(), afs, env, version, newFakeClient(srv)); err != nil {
			t.Fatalf("EnsureDocs: %v", err)
		}

		assertFileContent(t, afs, filepath.Join(dir, ".etag"), `"v1"`)

		data, err := fsext.ReadFile(afs, filepath.Join(dir, ".last_check"))
		if err != nil {
			t.Fatalf("ReadFile(.last_check): %v", err)
		}
		ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			t.Fatalf("ParseInt: %v", err)
		}
		if time.Since(time.Unix(ts, 0)) > time.Minute {
			t.Error(".last_check too old")
		}
	})

	type stalenessCase struct {
		name    string
		initial map[string]string
		// prepare runs after the first EnsureDocs call.
		prepare func(t *testing.T, state *fakeBundleState, afs fsext.Fs, dir string, srv *httptest.Server)
		verify  func(t *testing.T, afs fsext.Fs, dir string)
	}

	cases := []stalenessCase{
		{
			name:    "fresh cache serves from disk",
			initial: map[string]string{"doc.txt": "original"},
			prepare: nil,
			verify: func(t *testing.T, afs fsext.Fs, dir string) {
				t.Helper()
				assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "original")
			},
		},
		{
			name:    "stale same etag refreshes timestamp",
			initial: map[string]string{"doc.txt": "content"},
			prepare: func(t *testing.T, _ *fakeBundleState, afs fsext.Fs, dir string, _ *httptest.Server) {
				t.Helper()
				backdateLastCheck(t, afs, dir)
			},
			verify: func(t *testing.T, afs fsext.Fs, dir string) {
				t.Helper()
				data, err := fsext.ReadFile(afs, filepath.Join(dir, ".last_check"))
				if err != nil {
					t.Fatalf("ReadFile: %v", err)
				}
				ts, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
				if time.Since(time.Unix(ts, 0)) > time.Minute {
					t.Error(".last_check was not refreshed")
				}
				assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "content")
			},
		},
		{
			name:    "stale different etag redownloads",
			initial: map[string]string{"doc.txt": "old"},
			prepare: func(t *testing.T, state *fakeBundleState, afs fsext.Fs, dir string, _ *httptest.Server) {
				t.Helper()
				backdateLastCheck(t, afs, dir)
				state.content = map[string]string{"doc.txt": "new"}
				state.etag = `"v2"`
			},
			verify: func(t *testing.T, afs fsext.Fs, dir string) {
				t.Helper()
				assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "new")
				assertFileContent(t, afs, filepath.Join(dir, ".etag"), `"v2"`)
			},
		},
		{
			name:    "network error falls back to cache",
			initial: map[string]string{"doc.txt": "cached"},
			prepare: func(t *testing.T, _ *fakeBundleState, afs fsext.Fs, dir string, srv *httptest.Server) {
				t.Helper()
				backdateLastCheck(t, afs, dir)
				srv.Close()
			},
			verify: func(t *testing.T, afs fsext.Fs, dir string) {
				t.Helper()
				assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "cached")
			},
		},
		{
			name:    "missing last_check triggers check",
			initial: map[string]string{"doc.txt": "old"},
			prepare: func(t *testing.T, state *fakeBundleState, afs fsext.Fs, dir string, _ *httptest.Server) {
				t.Helper()
				_ = afs.Remove(filepath.Join(dir, ".last_check"))
				state.content = map[string]string{"doc.txt": "new"}
				state.etag = `"v2"`
			},
			verify: func(t *testing.T, afs fsext.Fs, dir string) {
				t.Helper()
				assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "new")
			},
		},
		{
			name:    "refresh GET failure preserves old cache",
			initial: map[string]string{"doc.txt": "good"},
			prepare: func(t *testing.T, state *fakeBundleState, afs fsext.Fs, dir string, _ *httptest.Server) {
				t.Helper()
				backdateLastCheck(t, afs, dir)
				state.etag = `"v2"`
				state.getStatus = http.StatusInternalServerError
			},
			verify: func(t *testing.T, afs fsext.Fs, dir string) {
				t.Helper()
				assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "good")
			},
		},
		{
			name:    "corrupt last_check self-heals",
			initial: map[string]string{"doc.txt": "good"},
			prepare: func(t *testing.T, _ *fakeBundleState, afs fsext.Fs, dir string, _ *httptest.Server) {
				t.Helper()
				if err := fsext.WriteFile(afs, filepath.Join(dir, ".last_check"), []byte("not-a-timestamp"), 0o640); err != nil {
					t.Fatalf("write corrupt last_check: %v", err)
				}
			},
			verify: func(t *testing.T, afs fsext.Fs, dir string) {
				t.Helper()
				assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "good")
			},
		},
		{
			name:    "corrupt etag self-heals",
			initial: map[string]string{"doc.txt": "good"},
			prepare: func(t *testing.T, state *fakeBundleState, afs fsext.Fs, dir string, _ *httptest.Server) {
				t.Helper()
				backdateLastCheck(t, afs, dir)
				if err := fsext.WriteFile(afs, filepath.Join(dir, ".etag"), []byte(""), 0o640); err != nil {
					t.Fatalf("write empty etag: %v", err)
				}
				state.content = map[string]string{"doc.txt": "updated"}
				state.etag = `"v2"`
			},
			verify: func(t *testing.T, afs fsext.Fs, dir string) {
				t.Helper()
				assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "updated")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv, state := newFakeBundleServer(t, tc.initial)
			afs := fsext.NewMemMapFs()
			env := map[string]string{"HOME": "/fakehome"}
			version := "test-staleness-" + strings.ReplaceAll(tc.name, " ", "-")

			client := newFakeClient(srv)
			dir, err := EnsureDocs(context.Background(), afs, env, version, client)
			if err != nil {
				t.Fatalf("EnsureDocs initial: %v", err)
			}

			if tc.prepare != nil {
				tc.prepare(t, state, afs, dir, srv)
			}

			got, err := EnsureDocs(context.Background(), afs, env, version, client)
			if err != nil {
				t.Fatalf("EnsureDocs second: %v", err)
			}
			if got != dir {
				t.Errorf("expected dir %q, got %q", dir, got)
			}
			if tc.verify != nil {
				tc.verify(t, afs, dir)
			}
		})
	}

	t.Run("corrupt bundle returns error and next run redownloads", func(t *testing.T) {
		t.Parallel()

		srv, state := newFakeBundleServer(t, map[string]string{"doc.txt": "good"})
		afs := fsext.NewMemMapFs()
		env := map[string]string{"HOME": "/fakehome"}
		version := "test-staleness-badextract"

		client := newFakeClient(srv)
		_, err := EnsureDocs(context.Background(), afs, env, version, client)
		if err != nil {
			t.Fatalf("EnsureDocs: %v", err)
		}
		backdateLastCheck(t, afs, filepath.Join("/fakehome/.local/share/k6/docs", version))

		// Server returns new ETag but corrupt body.
		state.etag = `"v2"`
		state.rawBody = []byte("not valid tar.zst")

		// Corrupt refresh must return an error.
		_, err = EnsureDocs(context.Background(), afs, env, version, client)
		if err == nil {
			t.Fatal("EnsureDocs should error on corrupt refresh")
		}

		// Server fixed — next call re-downloads successfully.
		state.rawBody = nil
		state.content = map[string]string{"doc.txt": "recovered"}
		state.etag = `"v3"`

		dir, err := EnsureDocs(context.Background(), afs, env, version, client)
		if err != nil {
			t.Fatalf("EnsureDocs recovery: %v", err)
		}
		assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "recovered")
	})

	t.Run("slow server times out instead of blocking", func(t *testing.T) {
		t.Parallel()

		goodSrv, _ := newFakeBundleServer(t, map[string]string{"doc.txt": "cached"})
		afs := fsext.NewMemMapFs()
		env := map[string]string{"HOME": "/fakehome"}
		version := "test-staleness-timeout"

		goodClient := newFakeClient(goodSrv)
		dir, err := EnsureDocs(context.Background(), afs, env, version, goodClient)
		if err != nil {
			t.Fatalf("EnsureDocs: %v", err)
		}
		backdateLastCheck(t, afs, dir)

		// Server that blocks until the request context is cancelled.
		slowSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		t.Cleanup(slowSrv.Close)
		slowClient := newFakeClient(slowSrv)

		start := time.Now()
		got, err := EnsureDocs(context.Background(), afs, env, version, slowClient)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("EnsureDocs should fall back to cache: %v", err)
		}
		if got != dir {
			t.Errorf("expected %q, got %q", dir, got)
		}
		if elapsed > 15*time.Second {
			t.Errorf("took %v, expected timeout around %v", elapsed, refreshTimeout)
		}
		assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "cached")
	})

	t.Run("corrupt last_check self-heals", func(t *testing.T) {
		t.Parallel()

		srv, _ := newFakeBundleServer(t, map[string]string{"doc.txt": "good"})
		afs := fsext.NewMemMapFs()
		env := map[string]string{"HOME": "/fakehome"}
		version := "test-corrupt-lastcheck"

		client := newFakeClient(srv)
		dir, err := EnsureDocs(context.Background(), afs, env, version, client)
		if err != nil {
			t.Fatalf("EnsureDocs: %v", err)
		}
		// Corrupt .last_check with non-numeric content.
		if err := fsext.WriteFile(afs, filepath.Join(dir, ".last_check"), []byte("not-a-timestamp"), 0o640); err != nil {
			t.Fatalf("write corrupt last_check: %v", err)
		}

		got, err := EnsureDocs(context.Background(), afs, env, version, client)
		if err != nil {
			t.Fatalf("EnsureDocs should self-heal corrupt last_check: %v", err)
		}
		if got != dir {
			t.Errorf("expected %q, got %q", dir, got)
		}
		assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "good")
	})

	t.Run("corrupt etag self-heals", func(t *testing.T) {
		t.Parallel()

		srv, state := newFakeBundleServer(t, map[string]string{"doc.txt": "good"})
		afs := fsext.NewMemMapFs()
		env := map[string]string{"HOME": "/fakehome"}
		version := "test-corrupt-etag"

		client := newFakeClient(srv)
		dir, err := EnsureDocs(context.Background(), afs, env, version, client)
		if err != nil {
			t.Fatalf("EnsureDocs: %v", err)
		}
		backdateLastCheck(t, afs, dir)

		// Truncate .etag to empty — mismatches every remote ETag.
		if err := fsext.WriteFile(afs, filepath.Join(dir, ".etag"), []byte(""), 0o640); err != nil {
			t.Fatalf("write empty etag: %v", err)
		}
		// Server still has same ETag — mismatched stored → re-download.
		state.content = map[string]string{"doc.txt": "updated"}
		state.etag = `"v2"`

		got, err := EnsureDocs(context.Background(), afs, env, version, client)
		if err != nil {
			t.Fatalf("EnsureDocs should self-heal corrupt etag: %v", err)
		}
		if got != dir {
			t.Errorf("expected %q, got %q", dir, got)
		}
		assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "updated")
	})
}

// --- fake HTTP server ---

type fakeBundleState struct {
	content   map[string]string
	etag      string
	getStatus int
	rawBody   []byte // if set, served instead of building a tar.zst from content
}

func newFakeBundleServer(t *testing.T, content map[string]string) (*httptest.Server, *fakeBundleState) {
	t.Helper()

	state := &fakeBundleState{content: content, etag: `"v1"`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if state.etag != "" {
			w.Header().Set("ETag", state.etag)
		}
		if r.Method == http.MethodHead {
			return
		}
		if state.getStatus != 0 {
			http.Error(w, "error", state.getStatus)
			return
		}
		if state.rawBody != nil {
			_, _ = w.Write(state.rawBody)
			return
		}
		archive := buildTarZst(t, state.content)
		_, _ = w.Write(archive.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv, state
}

func newLargeFileBundleServer(t *testing.T, name string, size int64) *httptest.Server {
	t.Helper()

	archive := buildTarZstLargeFile(t, name, size)
	data := archive.Bytes()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newMixedBundleServer(t *testing.T, entries []tarEntry, largeName string, largeSize int64) *httptest.Server {
	t.Helper()

	archive := buildTarZstMixed(t, entries, largeName, largeSize)
	data := archive.Bytes()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newFakeClient returns an *http.Client that redirects all requests to srv.
func newFakeClient(srv *httptest.Server) *http.Client {
	return &http.Client{Transport: &redirectTransport{
		base: srv.Client().Transport,
		to:   srv.URL,
	}}
}

// redirectTransport rewrites every request URL to a fixed target.
type redirectTransport struct {
	base http.RoundTripper
	to   string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u, err := url.Parse(t.to)
	if err != nil {
		return nil, err
	}
	clone.URL = u
	return t.base.RoundTrip(clone)
}

func backdateLastCheck(t *testing.T, afs fsext.Fs, dir string) {
	t.Helper()

	staleTime := time.Now().Add(-25 * time.Hour).Unix()
	f, err := afs.Create(filepath.Join(dir, ".last_check"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := f.Write([]byte(strconv.FormatInt(staleTime, 10))); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// --- archive builders ---

type tarEntry struct {
	name     string
	content  string
	typeflag byte
}

func buildTarZst(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()

	entries := make([]tarEntry, 0, len(files))
	for name, content := range files {
		entries = append(entries, tarEntry{name: name, content: content})
	}
	return buildTarZstRaw(t, entries)
}

func buildTarZstRaw(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd.NewWriter: %v", err)
	}
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Typeflag: e.typeflag,
			Name:     e.name,
			Mode:     0o644,
			Size:     int64(len(e.content)),
		}); err != nil {
			t.Fatalf("WriteHeader(%s): %v", e.name, err)
		}
		if _, err := tw.Write([]byte(e.content)); err != nil {
			t.Fatalf("Write(%s): %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Close: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zstd.Close: %v", err)
	}
	return &buf
}

func buildTarZstLargeFile(t *testing.T, name string, size int64) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd.NewWriter: %v", err)
	}
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: size}); err != nil {
		t.Fatalf("WriteHeader(%s): %v", name, err)
	}
	if _, err := io.CopyN(tw, zeros{}, size); err != nil {
		t.Fatalf("CopyN(%s): %v", name, err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Close: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zstd.Close: %v", err)
	}
	return &buf
}

func buildTarZstMixed(t *testing.T, entries []tarEntry, largeName string, largeSize int64) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd.NewWriter: %v", err)
	}
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.content))}); err != nil {
			t.Fatalf("WriteHeader(%s): %v", e.name, err)
		}
		if _, err := tw.Write([]byte(e.content)); err != nil {
			t.Fatalf("Write(%s): %v", e.name, err)
		}
	}
	if err := tw.WriteHeader(&tar.Header{Name: largeName, Mode: 0o644, Size: largeSize}); err != nil {
		t.Fatalf("WriteHeader(%s): %v", largeName, err)
	}
	if _, err := io.CopyN(tw, zeros{}, largeSize); err != nil {
		t.Fatalf("CopyN(%s): %v", largeName, err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Close: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zstd.Close: %v", err)
	}
	return &buf
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func assertFileContent(t *testing.T, afs fsext.Fs, path, want string) {
	t.Helper()

	data, err := fsext.ReadFile(afs, path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if got := string(data); got != want {
		t.Errorf("file %s content = %q, want %q", path, got, want)
	}
}
