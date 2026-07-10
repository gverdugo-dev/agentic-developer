package tui

import (
	"agentic-developer/internal/brand"

	"github.com/charmbracelet/lipgloss"
)

// The TUI styles, all derived from the shared brand palette. Bubble Tea owns
// the whole terminal in alt-screen mode, so unlike the plain CLI output there
// is no per-stream renderer here: the default renderer targets stdout, which
// is guaranteed to be a TTY (the TUI only launches on one).
var (
	// Intro animation.
	logoStyle         = lipgloss.NewStyle().Foreground(brand.Blue)
	logoFrontStyle    = lipgloss.NewStyle().Foreground(brand.Teal).Bold(true)
	logoScrambleStyle = lipgloss.NewStyle().Foreground(brand.Grey)
	subtitleStyle     = lipgloss.NewStyle().Foreground(brand.Grey)

	// Dashboard chrome.
	headerTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(brand.Blue)
	headerVersionStyle = lipgloss.NewStyle().Foreground(brand.Grey)
	panelTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(brand.BlueDark)
	footerStyle        = lipgloss.NewStyle().Foreground(brand.Grey)

	// Panels: the focused one gets the brand blue border so the eye always
	// knows where the keys go.
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(brand.Grey).Padding(0, 1)
	panelFocusedStyle = panelStyle.BorderForeground(brand.Blue)

	// List rows.
	itemStyle         = lipgloss.NewStyle()
	itemSelectedStyle = lipgloss.NewStyle().Foreground(brand.Teal).Bold(true)
	itemMutedStyle    = lipgloss.NewStyle().Foreground(brand.Grey)

	// Root input line and status.
	inputPromptStyle = lipgloss.NewStyle().Bold(true).Foreground(brand.Blue)
	errorTextStyle   = lipgloss.NewStyle().Foreground(brand.Red)
	spinnerStyle     = lipgloss.NewStyle().Foreground(brand.Teal)
)
