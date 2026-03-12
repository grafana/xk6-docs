package docs

import (
	"io"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown renders markdown content with ANSI styling for terminal display.
func renderMarkdown(w io.Writer, content string) error {
	r, err := glamour.NewTermRenderer(glamour.WithStylePath("dark"))
	if err != nil {
		return err
	}
	out, err := r.Render(content)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, out)
	return err
}
