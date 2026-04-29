package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	docs "github.com/grafana/xk6-docs/docs"
	"github.com/klauspost/compress/zstd"
	"github.com/rogpeppe/go-internal/testscript"
)

// buildTestBinary builds cmd/testk6 once and caches the result under the
// Go build cache directory. Subsequent runs reuse the cached binary if
// the source hasn't changed (checked via go build's own staleness logic).
func buildTestBinary(ctx context.Context, t *testing.T) string {
	t.Helper()

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("cache dir: %v", err)
	}
	dir := filepath.Join(cacheDir, "xk6-docs-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bin := filepath.Join(dir, "k6")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build testk6: %v\n%s", err, out)
	}
	return bin
}

func TestScripts(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bin := buildTestBinary(ctx, t)

	testscript.Run(t, testscript.Params{
		Dir: "testdata/scripts",
		Setup: func(env *testscript.Env) error {
			if err := os.Symlink(bin, filepath.Join(env.WorkDir, "k6")); err != nil {
				return err
			}
			return copyDir("testdata/cache", filepath.Join(env.WorkDir, "cache"))
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"ptyexec": func(ts *testscript.TestScript, neg bool, args []string) {
				runPtyExec(ctx, ts, neg, args)
			},
			"linecountgt":        runLinecountGt,
			"cpdir":              runCpdir,
			"bundlesrv":          runBundleSrv,
			"backdate-lastcheck": runBackdateLastcheck,
			"checkperm":          runCheckPerm,
			"mkzeroes":           runMkZeroes,
			"containslines":      runContainsLines,
			"setupautodetect": func(ts *testscript.TestScript, neg bool, args []string) {
				runSetupAutoDetect(ctx, ts, neg, args)
			},
		},
		UpdateScripts: os.Getenv("UPDATE_GOLDEN") != "",
	})
}

// runPtyExec runs a command with stdout attached to a PTY so the subprocess
// sees a real terminal (isatty returns true). Stderr is captured separately.
// Usage: ptyexec <prog> <args...>
func runPtyExec(ctx context.Context, ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 1 {
		ts.Fatalf("usage: ptyexec <prog> <args...>")
	}

	prog := ts.MkAbs(args[0])
	cmd := exec.CommandContext(ctx, prog, args[1:]...)
	cmd.Env = ptyEnv(ts)
	cmd.Dir = ts.Getenv("WORK")

	// Create PTY for stdout — makes isatty(stdout) true in the subprocess.
	ptmx, tty, err := pty.Open()
	if err != nil {
		ts.Fatalf("pty.Open: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	cmd.Stdout = tty
	cmd.Stdin = tty

	// Capture stderr separately via pipe so testscript stderr assertions work.
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = tty.Close()
		ts.Fatalf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		_ = tty.Close()
		ts.Fatalf("start: %v", err)
	}
	_ = tty.Close()

	// Read PTY master until slave closes (process exits).
	var stdoutBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, ptmx) // EIO on slave close is expected

	stderrBytes, _ := io.ReadAll(stderrPipe)

	waitErr := cmd.Wait()

	_, _ = ts.Stdout().Write(stdoutBuf.Bytes())
	_, _ = ts.Stderr().Write(stderrBytes)

	if neg {
		if waitErr == nil {
			ts.Fatalf("expected command to fail")
		}
	} else if waitErr != nil {
		ts.Fatalf("command failed: %v", waitErr)
	}
}

// ptyEnv returns the testscript environment plus TERM for TTY detection.
// It reads the unexported env field via reflect because TestScript has no
// method to enumerate all variables.
func ptyEnv(ts *testscript.TestScript) []string {
	v := reflect.ValueOf(ts).Elem().FieldByName("env")
	env := make([]string, v.Len(), v.Len()+1)
	for i := range v.Len() {
		env[i] = v.Index(i).String()
	}
	return append(env, "TERM=xterm-256color")
}

// runLinecountGt asserts that the first file has strictly more lines than the second.
// Usage: linecountgt <file-a> <file-b>
func runLinecountGt(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: linecountgt <file-a> <file-b>")
	}

	count := func(name string) int {
		data, err := os.ReadFile(ts.MkAbs(name))
		if err != nil {
			ts.Fatalf("read %s: %v", name, err)
		}
		n := 0
		for _, b := range data {
			if b == '\n' {
				n++
			}
		}
		return n
	}

	a, b := count(args[0]), count(args[1])
	if neg {
		if a > b {
			ts.Fatalf("%s (%d lines) > %s (%d lines), expected not greater", args[0], a, args[1], b)
		}
	} else {
		if a <= b {
			ts.Fatalf("%s (%d lines) <= %s (%d lines), expected greater", args[0], a, args[1], b)
		}
	}
}

// runCpdir recursively copies a directory tree.
// Usage: cpdir <src> <dst>
func runCpdir(ts *testscript.TestScript, _ bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: cpdir <src> <dst>")
	}
	if err := copyDir(ts.MkAbs(args[0]), ts.MkAbs(args[1])); err != nil {
		ts.Fatalf("cpdir: %v", err)
	}
}

// runBundleSrv starts a mock HTTP server that serves tar.zst bundles built
// from files in <dir>. Behavior is controlled via files in $WORK:
//   - .bundlesrv-etag: ETag header value (default: "v1")
//   - .bundlesrv-status: HTTP status code to return (overrides bundle serving)
//   - .bundlesrv-badpath: if present, inject a "../escape.txt" entry
//   - .bundlesrv-symlink: if present, inject a symlink entry
//   - .bundlesrv-oversize: if present, inject a 51MB file entry
//   - .bundlesrv-slow: if present, block on HEAD/GET until request cancelled
//   - .bundlesrv-raw-file: if present, serve this file's path as raw response body
//
// Sets K6_DOCS_BUNDLE_URL env var to the server URL.
// Usage: bundlesrv <dir>
func runBundleSrv(ts *testscript.TestScript, _ bool, args []string) {
	if len(args) < 1 {
		ts.Fatalf("usage: bundlesrv <dir>")
	}
	bundleDir := ts.MkAbs(args[0])
	workDir := ts.Getenv("WORK")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(filepath.Join(workDir, ".bundlesrv-slow")); err == nil {
			<-r.Context().Done()
			return
		}

		if _, err := os.Stat(filepath.Join(workDir, ".bundlesrv-raw-file")); err == nil {
			f, err := os.Open(filepath.Join(workDir, ".bundlesrv-raw-body"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer func() { _ = f.Close() }()
			w.Header().Set("ETag", `"v1"`)
			_, _ = io.Copy(w, f)
			return
		}

		if data, err := os.ReadFile(filepath.Join(workDir, ".bundlesrv-status")); err == nil {
			code, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if code > 0 {
				http.Error(w, "error", code)
				return
			}
		}

		etag := `"v1"`
		if data, err := os.ReadFile(filepath.Join(workDir, ".bundlesrv-etag")); err == nil {
			etag = strings.TrimSpace(string(data))
		}
		w.Header().Set("ETag", etag)

		if r.Method == http.MethodHead {
			return
		}

		var extras []tarEntry
		if _, err := os.Stat(filepath.Join(workDir, ".bundlesrv-badpath")); err == nil {
			extras = append(extras, tarEntry{name: "../escape.txt", content: []byte("evil")})
		}
		if _, err := os.Stat(filepath.Join(workDir, ".bundlesrv-symlink")); err == nil {
			extras = append(extras, tarEntry{name: "link.txt", typeflag: tar.TypeSymlink})
		}
		if _, err := os.Stat(filepath.Join(workDir, ".bundlesrv-oversize")); err == nil {
			extras = append(extras, tarEntry{name: "big.bin", oversizeBytes: 50<<20 + 1})
		}

		archive, err := buildTarZstFromDir(bundleDir, extras)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(archive)
	}))

	ts.Setenv("K6_DOCS_BUNDLE_URL", srv.URL)
}

// runBackdateLastcheck writes a stale timestamp (25h ago) to <dir>/.last_check.
// Usage: backdate-lastcheck <dir>
func runBackdateLastcheck(ts *testscript.TestScript, _ bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: backdate-lastcheck <dir>")
	}
	stale := time.Now().Add(-25 * time.Hour).Unix()
	path := filepath.Join(ts.MkAbs(args[0]), ".last_check")
	if err := os.WriteFile(path, []byte(strconv.FormatInt(stale, 10)), 0o640); err != nil {
		ts.Fatalf("backdate: %v", err)
	}
}

// runCheckPerm asserts that a file has the expected octal permission.
// Usage: checkperm <octal> <file>
func runCheckPerm(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: checkperm <octal> <file>")
	}
	want, err := strconv.ParseUint(args[0], 8, 32)
	if err != nil {
		ts.Fatalf("invalid octal %q: %v", args[0], err)
	}
	info, err := os.Stat(ts.MkAbs(args[1]))
	if err != nil {
		ts.Fatalf("stat %s: %v", args[1], err)
	}
	got := uint64(info.Mode().Perm())
	if neg {
		if got == want {
			ts.Fatalf("%s has permission %04o, expected different", args[1], got)
		}
	} else {
		if got != want {
			ts.Fatalf("%s has permission %04o, want %04o", args[1], got, want)
		}
	}
}

// runMkZeroes creates a sparse file of the given size.
// Usage: mkzeroes <size-bytes> <file>
func runMkZeroes(ts *testscript.TestScript, _ bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: mkzeroes <size-bytes> <file>")
	}
	size, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		ts.Fatalf("invalid size %q: %v", args[0], err)
	}
	f, err := os.Create(ts.MkAbs(args[1]))
	if err != nil {
		ts.Fatalf("create: %v", err)
	}
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		ts.Fatalf("truncate: %v", err)
	}
	_ = f.Close()
}

// runContainsLines asserts every non-empty line in <patterns-file> appears in <target-file>.
// Usage: containslines <patterns-file> <target-file>
func runContainsLines(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: containslines <patterns-file> <target-file>")
	}
	patterns, err := os.ReadFile(ts.MkAbs(args[0]))
	if err != nil {
		ts.Fatalf("read patterns: %v", err)
	}
	target, err := os.ReadFile(ts.MkAbs(args[1]))
	if err != nil {
		ts.Fatalf("read target: %v", err)
	}
	targetStr := string(target)
	for line := range strings.SplitSeq(string(patterns), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		found := strings.Contains(targetStr, line)
		if neg && found {
			ts.Fatalf("unexpected match for %q in %s", line, args[1])
		}
		if !neg && !found {
			ts.Fatalf("no match for %q in %s", line, args[1])
		}
	}
}

type tarEntry struct {
	name          string
	content       []byte
	typeflag      byte
	oversizeBytes int64
}

// buildTarZstFromDir walks a directory and builds a tar.zst archive from its files,
// then appends any extra entries (bad paths, symlinks, oversized files).
func buildTarZstFromDir(dir string, extras []tarEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(zw)

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{
			Name: rel,
			Mode: 0o644,
			Size: int64(len(content)),
		}); err != nil {
			return err
		}
		_, err = tw.Write(content)
		return err
	})
	if err != nil {
		return nil, err
	}

	for _, e := range extras {
		if err := writeExtraTarEntry(tw, e); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeExtraTarEntry(tw *tar.Writer, e tarEntry) error {
	if e.oversizeBytes > 0 {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o644, Size: e.oversizeBytes}); err != nil {
			return err
		}
		zeros := make([]byte, 32*1024)
		for written := int64(0); written < e.oversizeBytes; {
			n := min(e.oversizeBytes-written, int64(len(zeros)))
			if _, err := tw.Write(zeros[:n]); err != nil {
				return err
			}
			written += n
		}
		return nil
	}
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: e.typeflag,
		Name:     e.name,
		Mode:     0o644,
		Size:     int64(len(e.content)),
	}); err != nil {
		return err
	}
	_, err := tw.Write(e.content)
	return err
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

// runSetupAutoDetect creates a cache directory matching the k6 version
// detected from the test binary's build info. This makes the auto_detect
// test independent of which k6 version is in go.mod.
// Usage: setupautodetect <binary> <cache-dir>
func runSetupAutoDetect(ctx context.Context, ts *testscript.TestScript, _ bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: setupautodetect <binary> <cache-dir>")
	}
	binary := ts.MkAbs(args[0])
	cacheDir := ts.MkAbs(args[1])

	out, err := exec.CommandContext(ctx, "go", "version", "-m", binary).Output()
	if err != nil {
		ts.Fatalf("go version -m: %v", err)
	}

	k6ModPath := regexp.MustCompile(`^go\.k6\.io/k6(/v[1-9][0-9]*)?$`)
	var version string
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "dep" && k6ModPath.MatchString(fields[1]) {
			version = docs.VersionWildcard(fields[2])
			break
		}
	}
	if version == "" {
		ts.Fatalf("go.k6.io/k6 not found in build info of %s", binary)
	}

	dir := filepath.Join(cacheDir, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		ts.Fatalf("mkdir: %v", err)
	}

	data := fmt.Appendf(nil, `{"version":%q,"sections":[]}`, version)

	if err := os.WriteFile(filepath.Join(dir, "sections.json"), data, 0o644); err != nil {
		ts.Fatalf("write sections.json: %v", err)
	}

	ts.Setenv("K6_AUTO_VERSION", version)
	_, _ = fmt.Fprintf(ts.Stdout(), "%s\n", version)
}
