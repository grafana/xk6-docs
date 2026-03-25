package docs

import (
	"io"

	"github.com/charmbracelet/glamour"
	"github.com/muesli/termenv"
)

// renderMarkdown renders markdown content with ANSI styling for terminal display.
// It detects dark/light background from the destination writer and picks the
// matching glamour style automatically.
func renderMarkdown(w io.Writer, content string, width int) error {
	style := "dark"
	if !termenv.NewOutput(w).HasDarkBackground() {
		style = "light"
	}
	r, err := glamour.NewTermRenderer(glamour.WithStylePath(style), glamour.WithWordWrap(width))
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
