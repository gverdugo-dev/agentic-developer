// Package brand centralizes the adev color palette so the CLI output and the
// TUI stay visually consistent from a single source of truth.
//
// The palette comes from the project's report.css: blue is the primary accent,
// dark blue titles headings, teal is the secondary accent used for success and
// highlights. There is no brand red or yellow, so errors keep a semantic red
// and warnings a semantic amber. Truecolor hex values are downsampled by
// termenv on terminals with a smaller color profile.
package brand

import "github.com/charmbracelet/lipgloss"

// The adev palette.
const (
	Blue     = lipgloss.Color("#1a75bb")
	BlueDark = lipgloss.Color("#06538e")
	Teal     = lipgloss.Color("#5dc9be")
	Red      = lipgloss.Color("#d64550")
	Amber    = lipgloss.Color("#d69e2e")
	Grey     = lipgloss.Color("245")
)
