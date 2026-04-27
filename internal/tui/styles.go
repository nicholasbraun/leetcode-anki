package tui

import "github.com/charmbracelet/lipgloss"

const (
	glyphCursor = "▸"
	glyphSolved = "✓"
	glyphTried  = "~"
	glyphPaid   = "$"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFCC00")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7DD3FC")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F87171")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4ADE80")).
			Padding(0, 1)

	// rowSolvedStyle is successStyle without the horizontal padding so the
	// solved glyph occupies a single cell in list rows. The padded variant
	// is for banner-style "✓ Accepted" / "✓ Solved" labels.
	rowSolvedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4ADE80"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	easyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ADE80"))
	mediumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	hardStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))

	inProgressStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FBBF24"))

	breadcrumbStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	breadcrumbSepStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	breadcrumbActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7DD3FC"))

	dividerLineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	dividerAccentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7DD3FC"))

	footerKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("250"))

	footerLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	loadingStyleRun = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	loadingStyleSubmit = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("75"))
)

func difficultyStyle(d string) lipgloss.Style {
	switch d {
	case "EASY", "Easy":
		return easyStyle
	case "MEDIUM", "Medium":
		return mediumStyle
	case "HARD", "Hard":
		return hardStyle
	}
	return dimStyle
}
