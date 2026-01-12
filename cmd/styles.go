package cmd

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A020F0")). // Purple
		Bold(true).
		Padding(1, 0)

	promptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")) // Green

	warningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF4500")). // OrangeRed
		Bold(true)

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1E90FF")) // DodgerBlue
)
