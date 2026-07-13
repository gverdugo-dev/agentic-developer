package tui

import (
	"agentic-developer/internal/registry"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// This file drives the explore view (tab 6): searching the skills.sh
// registry, previewing a result's fetched SKILL.md, and installing it into a
// discovered harness config dir through the same picker and copier the local
// skill copy uses.
//
// Security stance, inherited from the CLI: a registry skill is a prompt the
// user's agent will execute, so "i" is only accepted after the fetched
// SKILL.md has been opened (enter), and the picked target still goes through
// an explicit y/N confirm before anything lands.

// exploreSearchMsg carries a finished registry search back into the program.
type exploreSearchMsg struct {
	query   string
	results []registry.Skill
	err     error
}

// exploreSearchCmd runs a registry search off the update loop.
func exploreSearchCmd(client registry.Client, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := client.Search(query)
		return exploreSearchMsg{query: query, results: results, err: err}
	}
}

// exploreFetchMsg carries a fetched registry skill back into the program.
type exploreFetchMsg struct {
	skill   registry.Skill
	fetched registry.Fetched
	err     error
}

// exploreFetchCmd downloads a registry skill's source off the update loop.
func exploreFetchCmd(client registry.Client, s registry.Skill) tea.Cmd {
	return func() tea.Msg {
		fetched, err := client.FetchSkill(s)
		return exploreFetchMsg{skill: s, fetched: fetched, err: err}
	}
}

// applyExploreSearch folds a search result into the model: an error lands in
// the status line (the view stays usable offline), results replace the list.
func (m dashModel) applyExploreSearch(msg exploreSearchMsg) dashModel {
	m.busy = false
	if msg.err != nil {
		m.status = errorTextStyle.Render("✗ search failed: " + msg.err.Error())
		return m
	}
	m.exploreQuery = msg.query
	m.exploreResults = msg.results
	if m.view == viewExploreTab {
		m.resetList()
	}
	m.status = itemSelectedStyle.Render("✓ ") + fmt.Sprintf("%d result(s) for %q", len(msg.results), msg.query)
	return m
}

// applyExploreFetch folds a fetch result into the model and opens the
// SKILL.md preview page. A refetch of the same skill replaces (and removes)
// the previous extraction.
func (m dashModel) applyExploreFetch(msg exploreFetchMsg) dashModel {
	m.busy = false
	if msg.err != nil {
		m.status = errorTextStyle.Render("✗ fetch failed: " + msg.err.Error())
		return m
	}
	if old, ok := m.exploreFetched[msg.skill.Ref()]; ok {
		os.RemoveAll(old.Root)
	}
	m.exploreFetched[msg.skill.Ref()] = msg.fetched
	m.page = &pageState{title: "registry: " + msg.skill.Name, content: exploreManifestPage(msg.skill, msg.fetched)}
	return m
}

// exploreOpen acts on enter in the explore view: the first time it fetches
// the skill's source (busy spinner), afterwards it opens the already-fetched
// SKILL.md page directly.
func (m dashModel) exploreOpen() (dashModel, tea.Cmd) {
	if m.selected >= len(m.exploreResults) {
		return m, nil
	}
	s := m.exploreResults[m.selected]
	if fetched, ok := m.exploreFetched[s.Ref()]; ok {
		m.page = &pageState{title: "registry: " + s.Name, content: exploreManifestPage(s, fetched)}
		return m, nil
	}
	m.busy = true
	return m, tea.Batch(exploreFetchCmd(m.reg, s), m.spin.Tick)
}

// requestExploreInstall drives "i" in the explore view. It refuses until the
// skill's SKILL.md has been fetched and shown (never install blind), then
// opens the same target picker the local skill copy uses; the picked target
// still confirms with y/N.
func (m dashModel) requestExploreInstall() (dashModel, tea.Cmd) {
	if m.selected >= len(m.exploreResults) || m.page != nil {
		return m, nil
	}
	s := m.exploreResults[m.selected]
	fetched, ok := m.exploreFetched[s.Ref()]
	if !ok {
		m.status = itemMutedStyle.Render("read it first: press enter to fetch its SKILL.md, then i to install")
		return m, nil
	}

	name := filepath.Base(fetched.Dir)
	targets := m.copyTargets(name)
	if len(targets) == 0 {
		if len(m.dirs) == 0 {
			m.status = itemMutedStyle.Render("no config dir discovered to install into")
		} else {
			m.status = itemMutedStyle.Render("every config dir already has " + name)
		}
		return m, nil
	}
	if len(targets) > 1 {
		var all []string
		for _, t := range targets {
			all = append(all, t.dirs...)
		}
		targets = append(targets, pickerTarget{label: fmt.Sprintf("all %d config dirs", len(all)), dirs: all})
	}

	m.picker = &pickerState{
		skillName:    name,
		srcPath:      fetched.Dir,
		targets:      targets,
		fromRegistry: true,
		source:       s.Source,
	}
	m.mode = modePicker
	return m, nil
}

// exploreEntryPreview builds the right-panel preview of one search result:
// its registry identity, and the fetched SKILL.md when it is already there.
func (m dashModel) exploreEntryPreview(i int) string {
	if i >= len(m.exploreResults) {
		return ""
	}
	s := m.exploreResults[i]

	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(s.Name) + "\n")
	b.WriteString(itemMutedStyle.Render("source   ") + s.Source + "\n")
	b.WriteString(itemMutedStyle.Render("installs ") + fmt.Sprintf("%d", s.Installs) + "\n")
	b.WriteString(itemMutedStyle.Render("ref      ") + s.Ref() + "\n\n")

	if fetched, ok := m.exploreFetched[s.Ref()]; ok {
		b.WriteString(itemMutedStyle.Render("SKILL.md fetched: press i to install") + "\n\n")
		b.WriteString(strings.TrimRight(fetched.Manifest, "\n"))
	} else {
		b.WriteString(itemMutedStyle.Render("press enter to fetch and read its SKILL.md"))
	}
	return b.String()
}

// exploreManifestPage builds the full-screen page of a fetched skill: the
// registry identity on top, then the SKILL.md exactly as it will land.
func exploreManifestPage(s registry.Skill, fetched registry.Fetched) string {
	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(s.Name) + itemMutedStyle.Render(" ("+s.Source+")") + "\n")
	b.WriteString(itemMutedStyle.Render(fmt.Sprintf("%d installs on skills.sh", s.Installs)) + "\n\n")
	b.WriteString(itemMutedStyle.Render("this is the SKILL.md your agent would execute; press esc, then i to install") + "\n\n")
	b.WriteString(strings.TrimRight(fetched.Manifest, "\n"))
	return b.String()
}

// exploreRowLabel renders one search result row: the name, its source repo
// and the registry install count.
func (m dashModel) exploreRowLabel(i, budget int, style lipgloss.Style) string {
	if i >= len(m.exploreResults) {
		return ""
	}
	s := m.exploreResults[i]
	suffix := " " + itemMutedStyle.Render(fmt.Sprintf("%s %d", s.Source, s.Installs))
	return style.Render(truncateTail(s.Name, budget-lipgloss.Width(suffix))) + suffix
}

// cleanupFetched removes the temp extraction roots of this session's
// registry fetches; Run calls it after the program exits.
func (m dashModel) cleanupFetched() {
	for _, fetched := range m.exploreFetched {
		os.RemoveAll(fetched.Root)
	}
}
