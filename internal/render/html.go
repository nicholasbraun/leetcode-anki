package render

import (
	"strings"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/charmbracelet/glamour"
)

// HTMLToMarkdown converts a LeetCode problem's HTML `content` into clean
// markdown suitable for rendering with glamour.
func HTMLToMarkdown(html string) (string, error) {
	conv := htmltomd.NewConverter("", true, nil)
	md, err := conv.ConvertString(html)
	if err != nil {
		return "", err
	}
	// LeetCode descriptions often use <pre> blocks that come out indented;
	// trim trailing whitespace per line to keep glamour rendering tidy.
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n"), nil
}

// MarkdownToTerminal renders the supplied markdown to ANSI-styled output
// word-wrapped to the given column width. width <= 0 disables wrapping.
func MarkdownToTerminal(md string, width int) (string, error) {
	opts := []glamour.TermRendererOption{glamour.WithAutoStyle()}
	if width > 0 {
		opts = append(opts, glamour.WithWordWrap(width))
	}
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return "", err
	}
	return r.Render(md)
}

// HTMLToTerminal is the convenience pipeline LeetCode descriptions take:
// HTML → markdown → glamour. Falls back to glamour-rendering the raw HTML
// if the markdown conversion fails — better to show messy text than nothing.
func HTMLToTerminal(html string, width int) (string, error) {
	md, err := HTMLToMarkdown(html)
	if err != nil {
		md = html
	}
	return MarkdownToTerminal(md, width)
}
