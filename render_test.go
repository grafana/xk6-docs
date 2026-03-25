package docs

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	t.Parallel()

	input := "# Hello\n\nSome **bold** text.\n"

	t.Run("renders with ANSI when enabled", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := renderMarkdown(&buf, input); err != nil {
			t.Fatalf("renderMarkdown: %v", err)
		}
		got := buf.String()
		// glamour output should contain ANSI escape sequences.
		if !strings.Contains(got, "\x1b[") {
			t.Errorf("expected ANSI escape codes in rendered output, got: %q", got)
		}
		// Content should still be present.
		if !strings.Contains(got, "Hello") {
			t.Errorf("expected 'Hello' in output, got: %q", got)
		}
		if !strings.Contains(got, "bold") {
			t.Errorf("expected 'bold' in output, got: %q", got)
		}
	})
}

func TestRenderIntegration(t *testing.T) {
	t.Parallel()

	afs, cacheDir := setupTestCache(t)
	gs := newTestGlobalState(t, afs)
	gs.Stdout.IsTTY = true
	gs.Flags.NoColor = false

	var stdoutBuf, cmdBuf bytes.Buffer
	gs.Stdout.Writer = &stdoutBuf

	cmd := newCmd(gs)
	cmd.SetOut(&cmdBuf)
	cmd.SetArgs([]string{"--cache-dir", cacheDir, "--version", "v0.55.x", "mod-a", "fn-one"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	// TTY mode: rendered output goes to gs.Stdout.Writer (stdoutBuf).
	got := stdoutBuf.String()
	if !strings.Contains(got, "modA.fnOne") {
		t.Errorf("expected topic content in rendered output, got: %s", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("expected ANSI escape codes from glamour, got: %s", got)
	}
}

func TestRenderSkippedWhenNoColor(t *testing.T) {
	t.Parallel()

	afs, cacheDir := setupTestCache(t)
	gs := newTestGlobalState(t, afs)
	gs.Stdout.IsTTY = true
	gs.Flags.NoColor = true

	var stdoutBuf, cmdBuf bytes.Buffer
	gs.Stdout.Writer = &stdoutBuf

	cmd := newCmd(gs)
	cmd.SetOut(&cmdBuf)
	cmd.SetArgs([]string{"--cache-dir", cacheDir, "--version", "v0.55.x", "mod-a", "fn-one"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	// NoColor: raw output goes to cmd buffer, not rendered.
	if !strings.Contains(cmdBuf.String(), "modA.fnOne(url)") {
		t.Errorf("expected raw output in cmd buffer, got: %s", cmdBuf.String())
	}
	if stdoutBuf.Len() > 0 {
		t.Errorf("expected renderer stdout to be empty when --no-color, got: %s", stdoutBuf.String())
	}
}

func TestPagerPipesRenderedOutput(t *testing.T) {
	t.Parallel()

	afs, cacheDir := setupTestCache(t)
	gs := newTestGlobalState(t, afs)
	gs.Stdout.IsTTY = false
	gs.Env["PAGER"] = "cat"

	var stdoutBuf, cmdBuf bytes.Buffer
	gs.Stdout.Writer = &stdoutBuf

	cmd := newCmd(gs)
	cmd.SetOut(&cmdBuf)
	cmd.SetArgs([]string{"-p", "--cache-dir", cacheDir, "--version", "v0.55.x", "mod-a", "fn-one"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	// Pager mode: glamour-rendered output piped through PAGER=cat to gs.Stdout.Writer.
	got := stdoutBuf.String()
	if !strings.Contains(got, "modA.fnOne") {
		t.Errorf("expected topic content, got: %s", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("expected ANSI escape codes, got: %s", got)
	}
	if cmdBuf.Len() > 0 {
		t.Errorf("expected no raw output in cmd buffer, got: %s", cmdBuf.String())
	}
}

func TestRenderSkippedWhenNonTTY(t *testing.T) {
	t.Parallel()

	afs, cacheDir := setupTestCache(t)
	gs := newTestGlobalState(t, afs)
	gs.Stdout.IsTTY = false

	var stdoutBuf, cmdBuf bytes.Buffer
	gs.Stdout.Writer = &stdoutBuf

	cmd := newCmd(gs)
	cmd.SetOut(&cmdBuf)
	cmd.SetArgs([]string{"--cache-dir", cacheDir, "--version", "v0.55.x", "mod-a", "fn-one"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	// Non-TTY: raw output goes to cmd buffer.
	if !strings.Contains(cmdBuf.String(), "modA.fnOne(url)") {
		t.Errorf("expected raw output in cmd buffer, got: %s", cmdBuf.String())
	}
	if stdoutBuf.Len() > 0 {
		t.Errorf("expected no rendered output when non-TTY, got: %s", stdoutBuf.String())
	}
}
