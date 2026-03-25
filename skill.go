package docs

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"

	"go.k6.io/k6/lib/fsext"
)

//go:embed skills/xk6-docs
var skillFiles embed.FS

const (
	skillSubdir       = "xk6-docs"
	binaryPlaceholder = "<binary>"
)

// installSkill copies the embedded skill files into destDir/xk6-docs/,
// replacing the <binary> placeholder in SKILL.md with the given path.
func installSkill(afs fsext.Fs, destDir, binaryPath string) error {
	skillDir := filepath.Join(destDir, skillSubdir)
	if err := afs.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("clean skill dir: %w", err)
	}

	return fs.WalkDir(skillFiles, "skills/xk6-docs", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel("skills/xk6-docs", path)
		if err != nil {
			return err
		}
		target := filepath.Join(skillDir, rel)

		if d.IsDir() {
			return afs.MkdirAll(target, 0o750)
		}

		data, err := skillFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		if d.Name() == "SKILL.md" {
			data = []byte(strings.ReplaceAll(string(data), binaryPlaceholder, binaryPath))
		}

		if err := afs.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}

		perm := fs.FileMode(0o600)
		if strings.HasSuffix(d.Name(), ".sh") {
			perm = 0o750
		}
		return fsext.WriteFile(afs, target, data, perm)
	})
}

type agentEntry struct {
	name      string
	globalDir string
}

func supportedAgents() []agentEntry {
	return []agentEntry{
		{"Claude Code", "~/.claude/skills/"},
		{"Cursor", "~/.agents/skills/"},
		{"Codex", "~/.agents/skills/"},
		{"Gemini CLI", "~/.agents/skills/"},
		{"GitHub Copilot", "~/.agents/skills/"},
		{"Amp", "~/.agents/skills/"},
		{"Cline", "~/.agents/skills/"},
		{"OpenCode", "~/.agents/skills/"},
		{"Windsurf", "~/.codeium/windsurf/skills/"},
		{"Roo Code", "~/.roo/skills/"},
		{"Goose", "~/.config/goose/skills/"},
	}
}

func skillHelpTable() string {
	var b strings.Builder
	b.WriteString("# Install the k6 docs agent skill\n\n")
	b.WriteString("Run this command with a skill directory to install:\n\n")
	b.WriteString("```\nk6 x docs skill <directory>\n```\n\n")
	b.WriteString("| Agent | Skill directory |\n")
	b.WriteString("|---|---|\n")
	for _, a := range supportedAgents() {
		_, _ = fmt.Fprintf(&b, "| %s | `%s` |\n", a.name, a.globalDir)
	}
	b.WriteString("\nMost agents that use `~/.agents/skills/` share the same directory.\n")
	return b.String()
}

func runSkill(afs fsext.Fs, w io.Writer, isTTY bool, binaryPath string, args []string) error {
	if len(args) == 0 {
		table := skillHelpTable()
		if isTTY {
			return renderMarkdown(w, table, 80)
		}
		_, err := io.WriteString(w, table)
		return err
	}

	destDir := args[0]

	absPath, err := resolveBinaryPath(binaryPath)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	if err := installSkill(afs, destDir, absPath); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(w, "Skill installed to %s\n", filepath.Join(destDir, skillSubdir))
	return nil
}

// resolveBinaryPath finds the absolute path of the binary.
// If binaryPath contains no directory separator (bare name like "k6"),
// it searches PATH via exec.LookPath. Otherwise resolves relative to cwd.
func resolveBinaryPath(binaryPath string) (string, error) {
	if filepath.Base(binaryPath) == binaryPath {
		found, err := exec.LookPath(binaryPath)
		if err != nil {
			return "", fmt.Errorf("look up %q in PATH: %w", binaryPath, err)
		}
		return filepath.EvalSymlinks(found)
	}

	abs, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
