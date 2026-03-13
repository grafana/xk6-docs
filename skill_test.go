package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.k6.io/k6/lib/fsext"
)

func TestSkillFilesEmbedded(t *testing.T) {
	t.Parallel()

	data, err := skillFiles.ReadFile("skills/xk6-docs/SKILL.md")
	if err != nil {
		t.Fatalf("SKILL.md not embedded: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("SKILL.md is empty")
	}

	entries, err := skillFiles.ReadDir("skills/xk6-docs/references")
	if err != nil {
		t.Fatalf("references dir not embedded: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no reference files embedded")
	}
}

func TestInstallSkill(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	destDir := t.TempDir()
	binaryPath := filepath.Join("usr", "local", "bin", "k6")

	if err := installSkill(afs, destDir, binaryPath); err != nil {
		t.Fatalf("installSkill: %v", err)
	}

	skillPath := filepath.Join(destDir, "xk6-docs", "SKILL.md")
	data, err := fsext.ReadFile(afs, skillPath)
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, binaryPath+" x docs") {
		t.Errorf("SKILL.md should contain %q", binaryPath+" x docs")
	}
	if strings.Contains(content, "<binary>") {
		t.Error("SKILL.md still contains '<binary>' placeholder")
	}

	httpRef := filepath.Join(destDir, "xk6-docs", "references", "http.md")
	if _, err := afs.Stat(httpRef); err != nil {
		t.Fatalf("http.md reference not installed: %v", err)
	}

	scriptPath := filepath.Join(destDir, "xk6-docs", "scripts", "validate-paths.sh")
	if _, err := afs.Stat(scriptPath); err != nil {
		t.Fatalf("validate-paths.sh not installed: %v", err)
	}
}

func TestInstallSkillOverwrites(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	destDir := t.TempDir()
	oldPath := filepath.Join("old", "path", "k6")
	newPath := filepath.Join("new", "path", "k6")

	if err := installSkill(afs, destDir, oldPath); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// Plant a stale file that shouldn't survive reinstall.
	staleFile := filepath.Join(destDir, "xk6-docs", "stale.txt")
	if err := fsext.WriteFile(afs, staleFile, []byte("leftover"), 0o600); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	if err := installSkill(afs, destDir, newPath); err != nil {
		t.Fatalf("second install: %v", err)
	}

	data, err := fsext.ReadFile(afs, filepath.Join(destDir, "xk6-docs", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, newPath) {
		t.Error("SKILL.md should contain new binary path after overwrite")
	}
	if strings.Contains(content, oldPath) {
		t.Error("SKILL.md still contains old binary path after overwrite")
	}

	// Stale file should be gone.
	if _, err := afs.Stat(staleFile); err == nil {
		t.Error("stale file should have been removed during reinstall")
	}
}

func TestSkillHelpTable(t *testing.T) {
	t.Parallel()

	table := skillHelpTable()
	if table == "" {
		t.Fatal("skillHelpTable returned empty string")
	}
	for _, agent := range []string{"Claude Code", "Cursor", "Codex"} {
		if !strings.Contains(table, agent) {
			t.Errorf("help table missing agent %q", agent)
		}
	}
	if !strings.Contains(table, "~/.claude/skills/") {
		t.Error("help table missing ~/.claude/skills/ path")
	}
}

func TestSkillCommandNoArgs(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	gs := newTestGlobalState(t, afs)
	gs.CmdArgs = []string{os.Args[0]}

	cmd := newCmd(gs)
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"skill"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill command should not error with no args: %v", err)
	}

	if !strings.Contains(buf.String(), "Claude Code") {
		t.Error("skill help should show agent table")
	}
}

func TestSkillCommandWithDir(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	gs := newTestGlobalState(t, afs)
	gs.CmdArgs = []string{os.Args[0]}

	destDir := t.TempDir()

	cmd := newCmd(gs)
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"skill", destDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill command: %v", err)
	}

	data, err := fsext.ReadFile(afs, filepath.Join(destDir, "xk6-docs", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "<binary>") {
		t.Error("SKILL.md still contains '<binary>' placeholder")
	}
	if !strings.Contains(content, "x docs") {
		t.Error("SKILL.md should contain 'x docs' command syntax")
	}
}

func TestResolveBinaryPath(t *testing.T) {
	t.Parallel()

	t.Run("bare name resolves via PATH", func(t *testing.T) {
		t.Parallel()

		// "go" is always in PATH during tests.
		got, err := resolveBinaryPath("go")
		if err != nil {
			t.Fatalf("resolveBinaryPath(go): %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("expected absolute path, got %q", got)
		}
	})

	t.Run("bare name not in PATH errors", func(t *testing.T) {
		t.Parallel()

		_, err := resolveBinaryPath("nonexistent-binary-xyz-999")
		if err == nil {
			t.Fatal("expected error for missing binary")
		}
	})

	t.Run("relative path resolves to absolute", func(t *testing.T) {
		t.Parallel()

		got, err := resolveBinaryPath(os.Args[0])
		if err != nil {
			t.Fatalf("resolveBinaryPath: %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("expected absolute path, got %q", got)
		}
	})
}
