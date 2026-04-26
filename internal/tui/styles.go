package tui

import "github.com/charmbracelet/lipgloss"

const (
	glyphEasy   = "◔"
	glyphMedium = "◑"
	glyphHard   = "●"
	glyphCursor = "▸"
	glyphSolved = "✓"
	glyphTried  = "✎"
	glyphPaid   = "🔒"
	glyphSpin   = "⟳"
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
