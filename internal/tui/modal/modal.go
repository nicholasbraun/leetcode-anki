// Package modal renders the shared centered-panel-plus-footer styling used
// by the language picker and the post-Accepted rating prompt. It is a
// render-only helper: no state, no input handling, no key contract.
package modal

import "github.com/charmbracelet/lipgloss"

// borderColor is the rounded-border accent shared by every modal in the
// app. Kept private so callers can't accidentally drift the palette.
const borderColor = "#7DD3FC"

// Options is the input to Render. Footer is pre-rendered by the caller
// (typically via the tui package's footer helper) so this package stays
// independent of that package's footer format.
type Options struct {
	Body   string
	Width  int
	Height int
	PadV   int
	PadH   int
	Footer string
}

// Render composes a rounded-border panel around Body, centers it on a
// (Width x Height-1) canvas, and appends Footer on its own trailing line.
// The result is the same shape produced by the previous inline code in
// langPickerView and gradeModalView.
func Render(opts Options) string {
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(opts.PadV, opts.PadH).
		Render(opts.Body)

	placed := lipgloss.Place(opts.Width, opts.Height-1, lipgloss.Center, lipgloss.Center, panel)
	return placed + "\n" + opts.Footer
}
