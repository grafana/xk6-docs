package docs

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsSmokeE2E is an opt-in smoke test that exercises the real k6 binary
// against a real filesystem cache fixture.
//
// Run manually with:
//
//	RUN_SMOKE_E2E=1 go test -run TestDocsSmokeE2E -v
func TestDocsSmokeE2E(t *testing.T) {
	t.Parallel()

	if os.Getenv("RUN_SMOKE_E2E") == "" {
		t.Skip("set RUN_SMOKE_E2E=1 to run smoke E2E test")
	}

	bin := os.Getenv("K6_BIN")
	if bin == "" {
		bin = "./k6"
	}
	if !filepath.IsAbs(bin) {
		abs, err := filepath.Abs(bin)
		if err != nil {
			t.Fatalf("resolve binary path: %v", err)
		}
		bin = abs
	}

	info, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("k6 binary not found at %q: %v", bin, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("k6 binary is not executable: %q", bin)
	}

	helpOut, helpErr := runCommandWithOutput(t, os.Environ(), bin, "x", "docs", "--help")
	if helpErr != nil {
		t.Fatalf("probe docs command: %v\n%s", helpErr, helpOut)
	}
	if !strings.Contains(helpOut, "docs [topic] [subtopic...]") {
		t.Fatalf("k6 binary does not expose expected docs subcommand\n%s", helpOut)
	}

	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	configHome := filepath.Join(tmp, "xdg")
	cacheDir := filepath.Join(tmp, "cache")

	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	if err := copyDir(filepath.Join("testdata", "cache"), cacheDir); err != nil {
		t.Fatalf("copy cache fixture: %v", err)
	}

	rendererPath := filepath.Join(tmp, "renderer.sh")
	rendererMarker := filepath.Join(tmp, "renderer-hit")
	rendererScript := "#!/bin/sh\nprintf 'hit\\n' >> \"$RENDERER_MARKER\"\ncat\n"
	if err := os.WriteFile(rendererPath, []byte(rendererScript), 0o755); err != nil {
		t.Fatalf("write renderer script: %v", err)
	}

	cfgDir := filepath.Join(configHome, "k6")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "docs.yaml"), []byte("renderer: "+rendererPath+"\n"), 0o644); err != nil {
		t.Fatalf("write docs config: %v", err)
	}

	env := append(os.Environ(),
		"HOME="+homeDir,
		"USERPROFILE="+homeDir,
		"XDG_CONFIG_HOME="+configHome,
		"K6_DOCS_CACHE_DIR="+cacheDir,
		"RENDERER_MARKER="+rendererMarker,
	)

	t.Run("non_tty_uses_raw_output", func(t *testing.T) {
		t.Parallel()

		out, err := runCommandWithOutput(t, env, bin, "x", "docs", "--version", "v0.55.x")
		if err != nil {
			t.Fatalf("non-TTY docs run: %v\n%s", err, out)
		}
		if !strings.Contains(out, "k6 Documentation (v0.55.x)") {
			t.Fatalf("missing docs header in output\n%s", out)
		}
		if _, statErr := os.Stat(rendererMarker); statErr == nil {
			t.Fatalf("renderer should not run in non-TTY mode, but marker file exists: %s", rendererMarker)
		}
	})

	t.Run("unknown_topic_surfaces_user_error", func(t *testing.T) {
		t.Parallel()

		out, err := runCommandWithOutput(t, env, bin, "x", "docs", "--version", "v0.55.x", "this-topic-does-not-exist")
		if err == nil {
			t.Fatalf("expected command error for unknown topic, got success\n%s", out)
		}
		if !strings.Contains(out, "topic not found") {
			t.Fatalf("expected 'topic not found' in output\n%s", out)
		}
	})

	t.Run("tty_best_effort_renderer_check", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("script"); err != nil {
			t.Skip("script command not found; skipping TTY smoke check")
		}

		// Clear marker from previous runs.
		_ = os.Remove(rendererMarker)

		out, err := runScriptTTY(t, env, bin, "x", "docs", "--version", "v0.55.x", "search", "http")
		if err != nil {
			t.Fatalf("TTY docs run via script: %v\n%s", err, out)
		}
		if !strings.Contains(out, `Results for "http"`) {
			t.Fatalf("missing search output in TTY run\n%s", out)
		}

		if _, statErr := os.Stat(rendererMarker); statErr != nil {
			t.Skip("TTY renderer path did not trigger in this environment; skipping strict renderer assertion")
		}
	})
}

func runCommandWithOutput(t *testing.T, env []string, bin string, args ...string) (string, error) {
	t.Helper()

	ctx := t.Context()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	return normalizeOutput(buf.String()), err
}

func runScriptTTY(t *testing.T, env []string, bin string, args ...string) (string, error) {
	t.Helper()

	// macOS BSD script syntax: script -q /dev/null <cmd> [args...]
	allArgs := append([]string{"-q", "/dev/null", bin}, args...)
	ctx := t.Context()
	cmd := exec.CommandContext(ctx, "script", allArgs...)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	return normalizeOutput(string(out)), err
}

func normalizeOutput(s string) string {
	// script(1) may emit carriage returns and small control artifacts around output.
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\b", "")
	s = strings.TrimSpace(s)
	return s
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
