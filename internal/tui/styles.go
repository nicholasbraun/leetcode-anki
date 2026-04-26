package tui

import "github.com/charmbracelet/lipgloss"

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
