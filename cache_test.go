package docs

import (
	"archive/tar"
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"go.k6.io/k6/lib/fsext"
)

func TestDownloadURL(t *testing.T) {
	t.Parallel()

	got := downloadURL("v1.0.0")
	want := "https://github.com/grafana/xk6-subcommand-docs/releases/download/doc-bundles/docs-v1.0.0.tar.zst"
	if got != want {
		t.Errorf("downloadURL(v1.0.0) = %q, want %q", got, want)
	}
}

func TestCacheDir(t *testing.T) {
	t.Parallel()

	t.Run("HOME set", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{"HOME": "/somepath"}
		dir, err := CacheDir(env, "v1.2.3")
		if err != nil {
			t.Fatalf("CacheDir: %v", err)
		}
		if !strings.HasSuffix(dir, filepath.Join("k6", "docs", "v1.2.3")) {
			t.Errorf("CacheDir = %q, want suffix %q", dir, filepath.Join("k6", "docs", "v1.2.3"))
		}
	})

	t.Run("USERPROFILE fallback", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{"USERPROFILE": `C:\Users\me`}
		dir, err := CacheDir(env, "v1.2.3")
		if err != nil {
			t.Fatalf("CacheDir: %v", err)
		}
		if !strings.HasSuffix(dir, filepath.Join("k6", "docs", "v1.2.3")) {
			t.Errorf("CacheDir = %q, want suffix %q", dir, filepath.Join("k6", "docs", "v1.2.3"))
		}
	})

	t.Run("neither set", func(t *testing.T) {
		t.Parallel()

		_, err := CacheDir(map[string]string{}, "v1.2.3")
		if err == nil {
			t.Fatal("expected error when neither HOME nor USERPROFILE is set")
		}
	})
}

func TestIsCached(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}

	// A version that definitely doesn't exist should not be cached.
	if IsCached(afs, env, "nonexistent-version-xyz") {
		t.Error("IsCached returned true for a version that should not exist")
	}

	// Create the directory and check again.
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

func TestExtract(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()

	// Build a .tar.zst archive in memory with two files.
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

	// Verify extracted files.
	assertFileContent(t, afs, filepath.Join(dest, "readme.txt"), "hello world")
	assertFileContent(t, afs, filepath.Join(dest, "subdir", "nested.txt"), "nested content")
}

func TestExtractRejectsTraversal(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()

	archive := buildTarZstRaw(t, []tarEntry{
		{name: "../escape.txt", content: "evil"},
	})

	dest := "/tmp/traversal-test"
	if err := afs.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	err := extract(afs, archive, dest)
	if err == nil {
		t.Fatal("extract should reject path traversal, but returned nil")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Errorf("expected traversal error, got: %v", err)
	}
}

func TestExtractRejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()

	archive := buildTarZstRaw(t, []tarEntry{
		{name: "/etc/passwd", content: "evil"},
	})

	dest := "/tmp/abspath-test"
	if err := afs.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	err := extract(afs, archive, dest)
	if err == nil {
		t.Fatal("extract should reject absolute path, but returned nil")
	}
}

func TestEnsureDocs(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}
	version := "test-ensure-" + t.Name()

	dir, err := CacheDir(env, version)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}

	archive := buildTarZst(t, map[string]string{
		"doc.txt": "documentation content",
	})

	mock := &mockHTTPClient{
		body:       archive.Bytes(),
		statusCode: http.StatusOK,
	}

	got, err := EnsureDocs(afs, env, version, mock)
	if err != nil {
		t.Fatalf("EnsureDocs: %v", err)
	}

	if got != dir {
		t.Errorf("EnsureDocs returned %q, want %q", got, dir)
	}

	assertFileContent(t, afs, filepath.Join(dir, "doc.txt"), "documentation content")

	// Calling again should use cache (no second HTTP call).
	got2, err := EnsureDocs(afs, env, version, mock)
	if err != nil {
		t.Fatalf("EnsureDocs second call: %v", err)
	}
	if got2 != dir {
		t.Errorf("second EnsureDocs returned %q, want %q", got2, dir)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 HTTP call, got %d", mock.calls)
	}
}

func TestEnsureDocsRejectsOversizedFile(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}
	version := "test-oversize-" + t.Name()

	archive := buildTarZstLargeFile(t, "big.bin", maxFileSize+1)

	mock := &mockHTTPClient{
		body:       archive.Bytes(),
		statusCode: http.StatusOK,
	}

	_, err := EnsureDocs(afs, env, version, mock)
	if err == nil {
		t.Fatal("EnsureDocs should reject file exceeding maxFileSize, but returned nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("expected maximum size error, got: %v", err)
	}
}

func TestEnsureDocsPermissions(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}
	version := "test-perms-" + t.Name()

	dir, err := CacheDir(env, version)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}

	archive := buildTarZst(t, map[string]string{
		"topfile.txt":       "content",
		"subdir/nested.txt": "nested",
	})

	mock := &mockHTTPClient{
		body:       archive.Bytes(),
		statusCode: http.StatusOK,
	}

	if _, err := EnsureDocs(afs, env, version, mock); err != nil {
		t.Fatalf("EnsureDocs: %v", err)
	}

	// Directories should have 0750 permissions.
	dirInfo, err := afs.Stat(filepath.Join(dir, "subdir"))
	if err != nil {
		t.Fatalf("Stat(subdir): %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o750 {
		t.Errorf("directory permission = %04o, want 0750", got)
	}

	// Files should have 0640 permissions.
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

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}
	version := "test-cleanup-" + t.Name()

	dir, err := CacheDir(env, version)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}

	// Archive with a valid file followed by an oversized file.
	// The valid file extracts first, then the oversized file causes an error.
	archive := buildTarZstMixed(t, []tarEntry{
		{name: "valid.txt", content: "ok"},
	}, "oversized.bin", maxFileSize+1)

	mock := &mockHTTPClient{
		body:       archive.Bytes(),
		statusCode: http.StatusOK,
	}

	_, err = EnsureDocs(afs, env, version, mock)
	if err == nil {
		t.Fatal("EnsureDocs should fail on oversized file")
	}

	// The cache directory should have been cleaned up.
	_, statErr := afs.Stat(dir)
	if statErr == nil {
		t.Errorf("cache directory %q still exists after failed extraction", dir)
	}
}

func TestEnsureDocsHTTPError(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}
	version := "test-ensure-httperr-" + t.Name()

	mock := &mockHTTPClient{
		body:       []byte("not found"),
		statusCode: http.StatusNotFound,
	}

	_, err := EnsureDocs(afs, env, version, mock)
	if err == nil {
		t.Fatal("EnsureDocs should fail on HTTP 404")
	}
}

func TestMultiVersionCoexistence(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	env := map[string]string{"HOME": "/fakehome"}

	// Create archives for two different versions.
	archiveV1 := buildTarZst(t, map[string]string{
		"doc.txt": "version one content",
	})
	archiveV2 := buildTarZst(t, map[string]string{
		"doc.txt": "version two content",
	})

	// Cache version 1.
	mockV1 := &mockHTTPClient{body: archiveV1.Bytes(), statusCode: http.StatusOK}
	dirV1, err := EnsureDocs(afs, env, "v1.0.x", mockV1)
	if err != nil {
		t.Fatalf("EnsureDocs v1: %v", err)
	}

	// Cache version 2.
	mockV2 := &mockHTTPClient{body: archiveV2.Bytes(), statusCode: http.StatusOK}
	dirV2, err := EnsureDocs(afs, env, "v2.0.x", mockV2)
	if err != nil {
		t.Fatalf("EnsureDocs v2: %v", err)
	}

	// Dirs should be different.
	if dirV1 == dirV2 {
		t.Fatal("different versions should have different cache dirs")
	}

	// Both should still exist and have correct content.
	assertFileContent(t, afs, filepath.Join(dirV1, "doc.txt"), "version one content")
	assertFileContent(t, afs, filepath.Join(dirV2, "doc.txt"), "version two content")

	// Both should be reported as cached.
	if !IsCached(afs, env, "v1.0.x") {
		t.Error("v1.0.x should be cached")
	}
	if !IsCached(afs, env, "v2.0.x") {
		t.Error("v2.0.x should be cached")
	}
}

// --- helpers ---

type mockHTTPClient struct {
	body       []byte
	statusCode int
	calls      int
}

func (m *mockHTTPClient) Get(_ string) (*http.Response, error) {
	m.calls++
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(bytes.NewReader(m.body)),
	}, nil
}

type tarEntry struct {
	name    string
	content string
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
		hdr := &tar.Header{
			Name: e.name,
			Mode: 0o644,
			Size: int64(len(e.content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
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

// buildTarZstLargeFile creates a tar.zst archive containing a single file
// of the given size filled with zeros. The content compresses well, so the
// archive stays small in memory even for large sizes.
func buildTarZstLargeFile(t *testing.T, name string, size int64) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd.NewWriter: %v", err)
	}

	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o644,
		Size: size,
	}); err != nil {
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

// buildTarZstMixed creates a tar.zst archive with normal entries followed by
// one large zero-filled file. This is useful for testing failure midway
// through extraction.
func buildTarZstMixed(t *testing.T, entries []tarEntry, largeName string, largeSize int64) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd.NewWriter: %v", err)
	}

	tw := tar.NewWriter(zw)

	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: e.name,
			Mode: 0o644,
			Size: int64(len(e.content)),
		}); err != nil {
			t.Fatalf("WriteHeader(%s): %v", e.name, err)
		}
		if _, err := tw.Write([]byte(e.content)); err != nil {
			t.Fatalf("Write(%s): %v", e.name, err)
		}
	}

	if err := tw.WriteHeader(&tar.Header{
		Name: largeName,
		Mode: 0o644,
		Size: largeSize,
	}); err != nil {
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

// zeros is an io.Reader that produces an endless stream of zero bytes.
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
