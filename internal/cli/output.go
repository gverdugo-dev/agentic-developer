package cli

import (
	"agentic-developer/internal/brand"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Two renderers bound to the actual output streams so lipgloss detects each
// stream's color support independently. Both honor NO_COLOR and a non-TTY
// destination (a pipe or file), degrading to plain text, so styled output stays
// safe to parse in scripts and CI.
var (
	outRenderer = lipgloss.NewRenderer(os.Stdout)
	errRenderer = lipgloss.NewRenderer(os.Stderr)

	// stdout styles.
	titleStyle   = outRenderer.NewStyle().Bold(true).Foreground(brand.BlueDark)
	commandStyle = outRenderer.NewStyle().Bold(true).Foreground(brand.Blue)
	accentStyle  = outRenderer.NewStyle().Foreground(brand.Blue)
	mutedStyle   = outRenderer.NewStyle().Foreground(brand.Grey)
	successStyle = outRenderer.NewStyle().Bold(true).Foreground(brand.Teal)

	// stderr styles.
	errorStyle = errRenderer.NewStyle().Bold(true).Foreground(brand.Red)
)

// accent styles a value the eye should land on: an artifact name, a harness, a
// version. Used inline inside messages built by the commands.
func accent(s string) string { return accentStyle.Render(s) }

// muted styles secondary detail, like a filesystem path.
func muted(s string) string { return mutedStyle.Render(s) }

// command styles a CLI subcommand word in help output.
func command(s string) string { return commandStyle.Render(s) }

// title styles a top-level heading.
func title(s string) string { return titleStyle.Render(s) }

// printSuccess writes a check-marked confirmation line to stdout.
func printSuccess(format string, a ...any) {
	fmt.Println(successStyle.Render("✓") + " " + fmt.Sprintf(format, a...))
}

// printInfo writes a plain status line to stdout (no symbol), for progress
// updates that are neither a final success nor an error.
func printInfo(format string, a ...any) {
	fmt.Println(fmt.Sprintf(format, a...))
}

// RenderError writes a red, cross-marked error line to stderr. main uses it as
// the single place that presents a failed command to the user.
func RenderError(err error) {
	fmt.Fprintln(os.Stderr, errorStyle.Render("✗ adev:")+" "+err.Error())
}
