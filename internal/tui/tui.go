// Package tui is the interactive terminal UI of adev, in the spirit of lazygit:
// running `adev` with no arguments on a terminal opens it. It is built on
// Bubble Tea, with a single root model that owns the screen and delegates to
// one sub-model per app state: the intro animation first, then the dashboard.
//
// The end goal of the TUI is discovery: scanning a folder tree for AI harness
// config dirs (.claude, .codex, .opencode) and analyzing what lives in each
// (skills, plugins, duplicates). This phase ships the foundation: the intro
// and the panel skeleton the discovery results will populate.
package tui

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// appState selects which sub-model owns the screen.
type appState int

// The app states, in the order the user sees them.
const (
	stateIntro appState = iota
	stateDashboard
)

// rootModel is the top-level Bubble Tea model: it tracks the terminal size,
// the current app state, and the sub-model of each state. Global concerns
// (quit keys, resize, state transitions) live here; everything else is
// delegated.
type rootModel struct {
	state  appState
	width  int
	height int
	intro  introModel
	dash   dashModel
}

// Run starts the TUI and blocks until the user quits. version is shown in the
// dashboard header, and scanRoot is the folder the initial discovery scan
// walks (typically the caller's working directory).
func Run(version, scanRoot string) error {
	// ADEV_TUI_LOG routes bubbletea's debug log (and our key traces) to a
	// file, since a TUI owns the terminal and cannot print diagnostics.
	if path := os.Getenv("ADEV_TUI_LOG"); path != "" {
		f, err := tea.LogToFile(path, "adev")
		if err != nil {
			return err
		}
		defer f.Close()
	}

	root := rootModel{
		state: stateIntro,
		intro: newIntro(),
		dash:  newDash(version, scanRoot),
	}

	_, err := tea.NewProgram(root, tea.WithAltScreen()).Run()
	return err
}

// Init kicks off the intro animation ticker and the initial discovery scan in
// parallel, so the results are often already there when the intro ends.
func (m rootModel) Init() tea.Cmd {
	return tea.Batch(introTick(), scanCmd(m.dash.pendingRoot), m.dash.spin.Tick)
}

// Update routes messages: global keys and resize are handled here, everything
// else goes to the sub-model of the current state.
func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.dash.setSize(msg.Width, msg.Height)
		return m, nil

	case introTickMsg:
		if m.state != stateIntro {
			return m, nil
		}
		if m.intro.advance() {
			m.state = stateDashboard
			return m, nil
		}
		return m, introTick()

	case tea.KeyMsg:
		if os.Getenv("ADEV_TUI_LOG") != "" {
			log.Printf("key: type=%d str=%q state=%d mode=%d", msg.Type, msg.String(), m.state, m.dash.mode)
		}
		// ctrl+c always quits, whatever the state.
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.state == stateIntro {
			// Any key skips the animation.
			m.state = stateDashboard
			return m, nil
		}
	}

	// Everything else (keys, scan results, spinner ticks, cursor blinks) is
	// the dashboard's business, whichever state is on screen: results that
	// arrive during the intro must not be lost.
	var cmd tea.Cmd
	m.dash, cmd = m.dash.Update(msg)
	return m, cmd
}

// View renders the current state, centering the intro on the full screen.
func (m rootModel) View() string {
	switch m.state {
	case stateIntro:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.intro.view())
	default:
		return m.dash.render()
	}
}
