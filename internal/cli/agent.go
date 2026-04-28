package cli

import (
	_ "embed"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

//go:embed agent_guide.md
var agentGuide string

// printAgentGuide prints a self-contained guide for AI agents, pointing them
// to the cached docs directory so they can read files directly.
// It reads the embedded SKILL.md, strips YAML frontmatter, and replaces
// the <dir> placeholder with the actual markdown path.
func printAgentGuide(w io.Writer, cacheDir string) {
	content := agentGuide
	dir := filepath.Join(cacheDir, "markdown")
	_, _ = fmt.Fprint(w, strings.ReplaceAll(content, "<dir>", dir))
}
