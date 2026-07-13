package tui

import (
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// logoLines is the adev wordmark in FIGlet's ANSI Shadow font. Every line has
// the same visible width, so the animation can treat the logo as a rune grid.
var logoLines = []string{
	" █████╗ ██████╗ ███████╗██╗   ██╗",
	"██╔══██╗██╔══██╗██╔════╝██║   ██║",
	"███████║██║  ██║█████╗  ██║   ██║",
	"██╔══██║██║  ██║██╔══╝  ╚██╗ ██╔╝",
	"██║  ██║██████╔╝███████╗ ╚████╔╝ ",
	"╚═╝  ╚═╝╚═════╝ ╚══════╝  ╚═══╝  ",
}

// subtitle appears under the logo once it has fully resolved.
const subtitle = "agentic developer"

// scrambleGlyphs is the pool unresolved cells cycle through while decoding.
// Only single-width box-drawing and block glyphs, so the grid never shifts.
var scrambleGlyphs = []rune("█▓▒░▀▄╔╗╚╝║═╬╠╣╦╩")

// Animation timing. The resolution front sweeps left to right one column per
// tick, plus a per-cell random jitter so the edge looks organic instead of a
// hard vertical line. With a 33-column logo the whole decode lands around
// 1.3s, and the hold keeps the finished logo briefly on screen before the
// dashboard. Any key skips all of it.
const (
	introTickInterval = 30 * time.Millisecond
	introColDelay     = 1  // ticks between adjacent columns resolving
	introMaxJitter    = 12 // max random ticks added to a cell's resolve time
	introFrontWidth   = 3  // ticks a just-resolved cell glows as the front
	introHoldFrames   = 20 // ticks the finished logo holds before moving on
)

// introTickMsg advances the animation one frame.
type introTickMsg struct{}

// introTick schedules the next animation frame.
func introTick() tea.Cmd {
	return tea.Tick(introTickInterval, func(time.Time) tea.Msg {
		return introTickMsg{}
	})
}

// introModel is the decode animation: every non-space cell of the logo shows
// churning random glyphs until its resolve frame passes, glows teal while the
// resolution front crosses it, and settles into the real glyph in brand blue.
type introModel struct {
	logo      [][]rune // the target grid
	resolveAt [][]int  // frame at which each cell fixes its real glyph
	scramble  [][]rune // current random glyph of each unresolved cell
	frame     int
	lastFrame int // frame at which the animation auto-finishes
}

// newIntro builds the grids and assigns each cell its resolve frame.
func newIntro() introModel {
	m := introModel{
		logo:      make([][]rune, len(logoLines)),
		resolveAt: make([][]int, len(logoLines)),
		scramble:  make([][]rune, len(logoLines)),
	}

	maxResolve := 0
	for r, line := range logoLines {
		runes := []rune(line)
		m.logo[r] = runes
		m.resolveAt[r] = make([]int, len(runes))
		m.scramble[r] = make([]rune, len(runes))

		for c, ch := range runes {
			if ch == ' ' {
				continue
			}
			at := c*introColDelay + rand.Intn(introMaxJitter)
			m.resolveAt[r][c] = at
			m.scramble[r][c] = randomGlyph()
			if at > maxResolve {
				maxResolve = at
			}
		}
	}

	m.lastFrame = maxResolve + introHoldFrames
	return m
}

// advance moves the animation one frame, re-rolling the glyph of every still
// unresolved cell, and reports whether the animation has finished.
func (m *introModel) advance() bool {
	m.frame++
	for r, line := range m.logo {
		for c, ch := range line {
			if ch != ' ' && m.frame < m.resolveAt[r][c] {
				m.scramble[r][c] = randomGlyph()
			}
		}
	}
	return m.frame >= m.lastFrame
}

// view renders the current frame plus the subtitle line. The subtitle line is
// always emitted (empty until the logo resolves) so the block height stays
// constant and the vertical centering never jumps.
func (m introModel) view() string {
	var b strings.Builder

	for r, line := range m.logo {
		for c, ch := range line {
			if ch == ' ' {
				b.WriteRune(' ')
				continue
			}
			age := m.frame - m.resolveAt[r][c]
			switch {
			case age < 0:
				b.WriteString(logoScrambleStyle.Render(string(m.scramble[r][c])))
			case age < introFrontWidth:
				b.WriteString(logoFrontStyle.Render(string(ch)))
			default:
				b.WriteString(logoStyle.Render(string(ch)))
			}
		}
		b.WriteRune('\n')
	}

	sub := ""
	if m.frame >= m.lastFrame-introHoldFrames {
		sub = subtitleStyle.Render(subtitle)
	}
	logoWidth := len([]rune(logoLines[0]))
	b.WriteString("\n" + lipgloss.PlaceHorizontal(logoWidth, lipgloss.Center, sub))

	return b.String()
}

// randomGlyph picks one glyph from the scramble pool.
func randomGlyph() rune {
	return scrambleGlyphs[rand.Intn(len(scrambleGlyphs))]
}
