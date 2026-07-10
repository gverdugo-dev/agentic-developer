package tui

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/doctor"
	"agentic-developer/internal/manage"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// dashPanel identifies the focusable panels of the dashboard.
type dashPanel int

// The panels, left to right.
const (
	panelPaths dashPanel = iota
	panelDetail
)

// dashMode selects who owns the keyboard: normal navigation, or the root
// input line at the bottom.
type dashMode int

// The dashboard modes.
const (
	modeNormal dashMode = iota
	modeInput
	modeConfirm
)

// actionKind is the mutation a pendingAction performs.
type actionKind int

// The supported actions.
const (
	actionDelete actionKind = iota
	actionUninstall
	actionRemoveMarket
	actionToggle
)

// pendingAction is a mutation waiting for confirmation (or running). label
// is what the footer shows; path feeds filesystem deletes; key feeds the
// claude CLI operations; confirmName, when set, must be typed back to
// confirm (whole config dirs).
type pendingAction struct {
	kind        actionKind
	label       string
	path        string
	key         string
	enable      bool // actionToggle: the target state
	confirmName string
}

// question phrases the confirm prompt for the action.
func (a pendingAction) question() string {
	switch a.kind {
	case actionUninstall:
		return "uninstall " + a.key + "?"
	case actionRemoveMarket:
		return "remove marketplace " + a.key + "?"
	default:
		return "delete " + abbreviateHome(a.path) + "?"
	}
}

// actionResultMsg carries a finished mutation back into the program.
type actionResultMsg struct {
	label string
	err   error
}

// runAction executes a confirmed mutation off the update loop.
func runAction(a pendingAction) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch a.kind {
		case actionDelete:
			err = manage.DeleteArtifact(a.path)
		case actionUninstall:
			_, err = manage.UninstallPlugin(a.key)
		case actionRemoveMarket:
			_, err = manage.RemoveMarketplace(a.key)
		case actionToggle:
			_, err = manage.SetPluginEnabled(a.key, a.enable)
		}
		return actionResultMsg{label: a.label, err: err}
	}
}

// dashView selects which list the dashboard browses: the discovered config
// dirs, one of the artifact-centric aggregations, or the doctor findings.
// Switched with 1-5.
type dashView int

// The views, in tab order.
const (
	viewPathsTab dashView = iota
	viewSkillsTab
	viewPluginsTab
	viewMarketsTab
	viewDoctorTab
)

// viewNames labels the tabs, indexed by dashView.
var viewNames = []string{"paths", "skills", "plugins", "marketplaces", "doctor"}

// artifactKind tags an entry of a drilled config dir.
type artifactKind int

// The artifact kinds inside a config dir.
const (
	refSkill artifactKind = iota
	refPlugin
	refMarket
)

// artifactRef points at one artifact of the drilled config dir.
type artifactRef struct {
	kind  artifactKind
	index int
}

// pageState is an open full-screen detail page, with its own scroll.
type pageState struct {
	title   string
	content string // styled, unwrapped; wrapped at render time
	offset  int
}

// scanResultMsg carries a finished discovery scan (and the doctor findings
// computed over it) back into the program.
type scanResultMsg struct {
	root     string
	dirs     []discovery.ConfigDir
	findings []doctor.Finding
	err      error
}

// scanCmd runs a discovery scan off the update loop, so a large tree never
// freezes the UI. The doctor checks run here too: they read artifact files
// from disk, which is just as much not the update loop's business.
func scanCmd(root string) tea.Cmd {
	return func() tea.Msg {
		dirs, err := discovery.Scan(root)
		var findings []doctor.Finding
		if err == nil {
			findings = doctor.Check(dirs)
		}
		return scanResultMsg{root: root, dirs: dirs, findings: findings, err: err}
	}
}

// dashModel is the lazygit-style dashboard: a browse list on the left
// (config dirs, or aggregated skills/plugins/marketplaces, per the active
// view), a preview panel on the right, and enter to drill in or open the
// full detail page. The footer doubles as the input line for changing the
// root ("o").
type dashModel struct {
	version  string
	width    int
	height   int
	focus    dashPanel
	selected int
	mode     dashMode
	view     dashView

	// drilled marks the paths view showing the artifacts of one config dir
	// (the one at drillParent) instead of the config dir list.
	drilled     bool
	drillParent int

	// page, when non-nil, is the full-screen detail page on top of the
	// browse layout.
	page *pageState

	// pending is the mutation waiting in the confirm prompt; busy marks one
	// running; status is the transient result line shown in the footer until
	// the next keypress.
	pending *pendingAction
	busy    bool
	status  string

	// pathsOffset is the scroll position of the browse list; it follows the
	// selection so the selected row is always visible. detailOffset is the
	// scroll position of the preview panel, moved with j/k while the panel
	// is focused and reset when the selection changes.
	pathsOffset  int
	detailOffset int

	// root is the folder the current results were scanned from; pendingRoot
	// is the scan in flight ("" when idle). Results for anything other than
	// pendingRoot are stale and dropped.
	root        string
	pendingRoot string
	dirs        []discovery.ConfigDir
	scanErr     error

	// The artifact-centric aggregations and the doctor findings, recomputed
	// on every scan result.
	skills   []discovery.SkillGroup
	plugins  []discovery.PluginGroup
	markets  []discovery.MarketplaceGroup
	findings []doctor.Finding

	input    textinput.Model
	inputErr error
	spin     spinner.Model
}

// newDash builds the dashboard primed to scan root: the caller schedules the
// matching scanCmd, so pendingRoot starts set.
func newDash(version, root string) dashModel {
	input := textinput.New()
	input.Placeholder = "/absolute/path"
	input.Prompt = ""

	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(spinnerStyle))

	return dashModel{
		version:     version,
		focus:       panelPaths,
		root:        root,
		pendingRoot: root,
		input:       input,
		spin:        spin,
	}
}

// setSize records the terminal size the layout is computed from. The input
// never spans the full line: room is reserved next to it so a validation
// error stays visible instead of being clipped at the terminal edge.
func (m *dashModel) setSize(width, height int) {
	m.width, m.height = width, height
	m.input.Width = width - lipgloss.Width(inputPrompt) - inputErrorReserve
	if m.input.Width < minInputWidth {
		m.input.Width = minInputWidth
	}
	m.ensureSelectedVisible()
}

// panelWidths returns the content widths of the two panels: a 1:2 split of
// the terminal, minus the columns the two borders eat.
func (m dashModel) panelWidths() (left, right int) {
	left = m.width / 3
	if left < minPanelWidth {
		left = minPanelWidth
	}
	right = m.width - left - 2*panelBorderLines
	if right < minPanelWidth {
		right = minPanelWidth
	}
	return left, right
}

// bodyHeight returns the content height of the panels.
func (m dashModel) bodyHeight() int {
	h := m.height - dashChromeLines - panelBorderLines
	if h < 1 {
		h = 1
	}
	return h
}

// listCapacity returns how many content rows fit in a panel below its title
// line and the blank line after it.
func (m dashModel) listCapacity() int {
	c := m.bodyHeight() - 2
	if c < 1 {
		c = 1
	}
	return c
}

// listLen returns the row count of the active browse list.
func (m dashModel) listLen() int {
	switch m.view {
	case viewPathsTab:
		if m.drilled {
			return len(m.drillRefs())
		}
		return len(m.dirs)
	case viewSkillsTab:
		return len(m.skills)
	case viewPluginsTab:
		return len(m.plugins)
	case viewMarketsTab:
		return len(m.markets)
	default:
		return len(m.findings)
	}
}

// drillRefs flattens the artifacts of the drilled config dir, skills first,
// then plugins, then marketplaces.
func (m dashModel) drillRefs() []artifactRef {
	if m.drillParent >= len(m.dirs) {
		return nil
	}
	dir := m.dirs[m.drillParent]

	refs := make([]artifactRef, 0, len(dir.Skills)+len(dir.Plugins)+len(dir.Marketplaces))
	for i := range dir.Skills {
		refs = append(refs, artifactRef{kind: refSkill, index: i})
	}
	for i := range dir.Plugins {
		refs = append(refs, artifactRef{kind: refPlugin, index: i})
	}
	for i := range dir.Marketplaces {
		refs = append(refs, artifactRef{kind: refMarket, index: i})
	}
	return refs
}

// ensureSelectedVisible scrolls the browse list just enough to keep the
// selected row inside the visible window, the way every lazy-style TUI
// follows its cursor.
func (m *dashModel) ensureSelectedVisible() {
	capacity := m.listCapacity()
	if m.selected < m.pathsOffset {
		m.pathsOffset = m.selected
	}
	if m.selected >= m.pathsOffset+capacity {
		m.pathsOffset = m.selected - capacity + 1
	}
	if m.pathsOffset < 0 {
		m.pathsOffset = 0
	}
}

// resetList moves the browse selection back to the top.
func (m *dashModel) resetList() {
	m.selected = 0
	m.pathsOffset = 0
	m.detailOffset = 0
}

// scrollDetail moves the preview panel by delta lines, clamped to its
// content.
func (m *dashModel) scrollDetail(delta int) {
	_, right := m.panelWidths()
	total := len(m.detailLines(right - 2))
	maxOffset := total - m.listCapacity()
	if maxOffset < 0 {
		maxOffset = 0
	}

	m.detailOffset += delta
	if m.detailOffset > maxOffset {
		m.detailOffset = maxOffset
	}
	if m.detailOffset < 0 {
		m.detailOffset = 0
	}
}

// scrollPage moves the open page by delta lines, clamped to its content.
func (m *dashModel) scrollPage(delta int) {
	if m.page == nil {
		return
	}
	total := len(wrapLines(m.page.content, m.pageInnerWidth()))
	maxOffset := total - m.listCapacity()
	if maxOffset < 0 {
		maxOffset = 0
	}

	m.page.offset += delta
	if m.page.offset > maxOffset {
		m.page.offset = maxOffset
	}
	if m.page.offset < 0 {
		m.page.offset = 0
	}
}

// pageInnerWidth returns the content width of the full-screen page panel.
func (m dashModel) pageInnerWidth() int {
	w := m.width - panelBorderLines - 2
	if w < minPanelWidth {
		w = minPanelWidth
	}
	return w
}

// Update handles every message the root delegates to the dashboard and
// returns the updated model plus any command to run.
func (m dashModel) Update(msg tea.Msg) (dashModel, tea.Cmd) {
	switch msg := msg.(type) {

	case scanResultMsg:
		if msg.root != m.pendingRoot {
			return m, nil
		}
		m.pendingRoot = ""
		m.root = msg.root
		m.dirs = msg.dirs
		m.scanErr = msg.err
		m.skills = discovery.GroupSkills(msg.dirs)
		m.plugins = discovery.GroupPlugins(msg.dirs)
		m.markets = discovery.GroupMarketplaces(msg.dirs)
		m.findings = msg.findings
		m.drilled = false
		m.page = nil
		if m.selected >= m.listLen() {
			m.selected = 0
		}
		m.pathsOffset = 0
		m.detailOffset = 0
		m.ensureSelectedVisible()
		return m, nil

	case actionResultMsg:
		m.busy = false
		if msg.err != nil {
			m.status = errorTextStyle.Render("✗ " + msg.err.Error())
			return m, nil
		}
		m.status = itemSelectedStyle.Render("✓ ") + msg.label
		// The world changed: rescan the current root to refresh every view.
		m.pendingRoot = m.root
		return m, tea.Batch(scanCmd(m.root), m.spin.Tick)

	case spinner.TickMsg:
		if m.pendingRoot == "" && !m.busy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch m.mode {
		case modeInput:
			return m.updateInputKey(msg)
		case modeConfirm:
			return m.updateConfirmKey(msg)
		}
		return m.updateNormalKey(msg)
	}

	// Non-key messages (cursor blinks) keep the input line alive.
	if m.mode == modeInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// updateNormalKey applies the navigation key map.
func (m dashModel) updateNormalKey(msg tea.KeyMsg) (dashModel, tea.Cmd) {
	key := msg.String()

	// Any key dismisses the last action's status line.
	m.status = ""

	// While a mutation runs, only quitting is allowed.
	if m.busy {
		if key == "q" {
			return m, tea.Quit
		}
		return m, nil
	}

	// An open detail page owns j/k/esc until it closes.
	if m.page != nil {
		switch key {
		case "q":
			return m, tea.Quit
		case "esc", "backspace":
			m.page = nil
		case "j", "down":
			m.scrollPage(1)
		case "k", "up":
			m.scrollPage(-1)
		}
		return m, nil
	}

	switch key {
	case "q":
		return m, tea.Quit

	case "esc", "backspace":
		if m.drilled {
			m.drilled = false
			m.resetList()
			m.selected = m.drillParent
			m.ensureSelectedVisible()
		}

	case "1", "2", "3", "4", "5":
		view := dashView(key[0] - '1')
		if view != m.view {
			m.view = view
			m.drilled = false
			m.focus = panelPaths
			m.resetList()
		}

	case "tab", "h", "l", "left", "right":
		if m.focus == panelPaths {
			m.focus = panelDetail
		} else {
			m.focus = panelPaths
		}

	case "j", "down":
		if m.focus == panelPaths {
			if m.selected < m.listLen()-1 {
				m.selected++
				m.detailOffset = 0
				m.ensureSelectedVisible()
			}
		} else {
			m.scrollDetail(1)
		}

	case "k", "up":
		if m.focus == panelPaths {
			if m.selected > 0 {
				m.selected--
				m.detailOffset = 0
				m.ensureSelectedVisible()
			}
		} else {
			m.scrollDetail(-1)
		}

	case "enter":
		return m.openSelected(), nil

	case "d":
		return m.requestDelete(), nil

	case "t":
		return m.requestToggle()

	case "o":
		m.mode = modeInput
		m.inputErr = nil
		m.input.SetValue(m.root)
		m.input.CursorEnd()
		return m, m.input.Focus()
	}

	return m, nil
}

// requestDelete resolves what "d" targets in the current view and opens the
// confirm prompt for it. Group rows are only deletable when they live in
// exactly one place; otherwise the paths view is where you pick which copy.
func (m dashModel) requestDelete() dashModel {
	if m.listLen() == 0 || m.page != nil {
		return m
	}
	if m.view == viewDoctorTab {
		m.status = itemMutedStyle.Render("findings are informational: fix them from the other views")
		return m
	}

	var action *pendingAction
	switch m.view {
	case viewPathsTab:
		if !m.drilled {
			dir := m.dirs[m.selected]
			display := m.displayPath(dir.Path)
			action = &pendingAction{
				kind:        actionDelete,
				label:       "deleted " + display,
				path:        dir.Path,
				confirmName: display,
			}
			break
		}
		action = m.drillDeleteAction(m.drillRefs()[m.selected])

	case viewSkillsTab:
		g := m.skills[m.selected]
		if len(g.Locations) > 1 {
			m.status = itemMutedStyle.Render(fmt.Sprintf("%s lives in %d places: delete it from the paths view", g.Name, len(g.Locations)))
			return m
		}
		s := g.Locations[0].Item
		action = &pendingAction{kind: actionDelete, label: "deleted skill " + s.Name, path: s.Path}

	case viewPluginsTab:
		g := m.plugins[m.selected]
		if len(g.Locations) > 1 {
			m.status = itemMutedStyle.Render(fmt.Sprintf("%s lives in %d places: delete it from the paths view", g.Key, len(g.Locations)))
			return m
		}
		action = pluginDeleteAction(g.Locations[0].Item)

	default:
		g := m.markets[m.selected]
		if len(g.Locations) > 1 {
			m.status = itemMutedStyle.Render(fmt.Sprintf("%s lives in %d places: delete it from the paths view", g.Name, len(g.Locations)))
			return m
		}
		action = marketplaceDeleteAction(g.Locations[0].Item)
	}

	if action == nil {
		return m
	}
	m.pending = action
	m.mode = modeConfirm
	m.inputErr = nil
	if action.confirmName != "" {
		m.input.SetValue("")
		m.input.Placeholder = action.confirmName
		m.input.Focus()
	}
	return m
}

// drillDeleteAction builds the delete action for one artifact of the
// drilled config dir.
func (m dashModel) drillDeleteAction(ref artifactRef) *pendingAction {
	dir := m.dirs[m.drillParent]
	switch ref.kind {
	case refSkill:
		s := dir.Skills[ref.index]
		return &pendingAction{kind: actionDelete, label: "deleted skill " + s.Name, path: s.Path}
	case refPlugin:
		return pluginDeleteAction(dir.Plugins[ref.index])
	default:
		return marketplaceDeleteAction(dir.Marketplaces[ref.index])
	}
}

// pluginDeleteAction deletes a folder plugin from disk, but a registry
// plugin through the claude CLI (uninstall), so the registry and its cache
// stay consistent.
func pluginDeleteAction(p discovery.Plugin) *pendingAction {
	if p.Marketplace != "" {
		key := p.Name + "@" + p.Marketplace
		return &pendingAction{kind: actionUninstall, label: "uninstalled " + key, key: key}
	}
	return &pendingAction{kind: actionDelete, label: "deleted plugin " + p.Name, path: p.Path}
}

// marketplaceDeleteAction removes a registry marketplace through the claude
// CLI, or a folder one from disk.
func marketplaceDeleteAction(mkt discovery.Marketplace) *pendingAction {
	if mkt.Source != "" {
		return &pendingAction{kind: actionRemoveMarket, label: "removed marketplace " + mkt.Name, key: mkt.Name}
	}
	return &pendingAction{kind: actionDelete, label: "deleted marketplace " + mkt.Name, path: mkt.Path}
}

// requestToggle flips the enabled state of the selected registry plugin
// (claude CLI). Toggling is not destructive, so it runs without confirm.
func (m dashModel) requestToggle() (dashModel, tea.Cmd) {
	if m.listLen() == 0 || m.page != nil {
		return m, nil
	}

	var target *discovery.Plugin
	switch {
	case m.view == viewPathsTab && m.drilled:
		ref := m.drillRefs()[m.selected]
		if ref.kind == refPlugin {
			target = &m.dirs[m.drillParent].Plugins[ref.index]
		}
	case m.view == viewPluginsTab:
		g := m.plugins[m.selected]
		if len(g.Locations) == 1 {
			target = &g.Locations[0].Item
		} else {
			m.status = itemMutedStyle.Render(fmt.Sprintf("%s lives in %d places: toggle it from the paths view", g.Key, len(g.Locations)))
			return m, nil
		}
	}

	if target == nil || target.Marketplace == "" {
		m.status = itemMutedStyle.Render("only installed registry plugins can be toggled")
		return m, nil
	}

	key := target.Name + "@" + target.Marketplace
	verb := "enabled "
	if target.Enabled {
		verb = "disabled "
	}
	m.busy = true
	return m, tea.Batch(runAction(pendingAction{
		kind:   actionToggle,
		label:  verb + key,
		key:    key,
		enable: !target.Enabled,
	}), m.spin.Tick)
}

// updateConfirmKey applies the confirm-prompt key map: y/n for normal
// deletes, the typed name plus enter for whole config dirs.
func (m dashModel) updateConfirmKey(msg tea.KeyMsg) (dashModel, tea.Cmd) {
	if m.pending == nil {
		m.mode = modeNormal
		return m, nil
	}

	// Whole config dirs require their displayed name typed back.
	if m.pending.confirmName != "" {
		switch msg.Type {
		case tea.KeyEsc:
			return m.cancelConfirm(), nil
		case tea.KeyEnter:
			if strings.TrimSpace(m.input.Value()) == m.pending.confirmName {
				return m.startAction()
			}
			m.inputErr = fmt.Errorf("name does not match")
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.inputErr = nil
		return m, cmd
	}

	switch msg.String() {
	case "y", "Y":
		return m.startAction()
	case "n", "N", "esc":
		return m.cancelConfirm(), nil
	}
	return m, nil
}

// cancelConfirm closes the confirm prompt without acting.
func (m dashModel) cancelConfirm() dashModel {
	m.pending = nil
	m.mode = modeNormal
	m.inputErr = nil
	m.input.Blur()
	m.input.Placeholder = "/absolute/path"
	return m
}

// startAction launches the confirmed mutation.
func (m dashModel) startAction() (dashModel, tea.Cmd) {
	action := *m.pending
	m = m.cancelConfirm()
	m.busy = true
	return m, tea.Batch(runAction(action), m.spin.Tick)
}

// openSelected acts on enter: in the paths view it drills into the selected
// config dir first, then opens the artifact's page; in the artifact views it
// opens the group's page directly.
func (m dashModel) openSelected() dashModel {
	if m.listLen() == 0 {
		return m
	}

	switch m.view {
	case viewPathsTab:
		if !m.drilled {
			m.drilled = true
			m.drillParent = m.selected
			m.resetList()
			return m
		}
		refs := m.drillRefs()
		if m.selected < len(refs) {
			title, content := m.artifactPage(refs[m.selected])
			m.page = &pageState{title: title, content: content}
		}

	case viewSkillsTab:
		g := m.skills[m.selected]
		m.page = &pageState{title: "skill: " + g.Name, content: skillGroupPreview(g)}

	case viewPluginsTab:
		g := m.plugins[m.selected]
		m.page = &pageState{title: "plugin: " + g.Key, content: pluginGroupPreview(g)}

	case viewMarketsTab:
		g := m.markets[m.selected]
		m.page = &pageState{title: "marketplace: " + g.Name, content: marketplaceGroupPreview(g)}

	default:
		f := m.findings[m.selected]
		m.page = &pageState{title: "finding: " + f.Check, content: findingPreview(f)}
	}

	return m
}

// artifactPage builds the page of one artifact of the drilled config dir.
func (m dashModel) artifactPage(ref artifactRef) (title, content string) {
	dir := m.dirs[m.drillParent]
	switch ref.kind {
	case refSkill:
		s := dir.Skills[ref.index]
		return "skill: " + s.Name, skillPreview(s)
	case refPlugin:
		p := dir.Plugins[ref.index]
		return "plugin: " + p.Name, pluginPreview(p)
	default:
		mkt := dir.Marketplaces[ref.index]
		return "marketplace: " + mkt.Name, marketplacePreview(mkt)
	}
}

// updateInputKey applies the root-input key map: enter validates and
// launches the scan, esc cancels, everything else edits the input.
func (m dashModel) updateInputKey(msg tea.KeyMsg) (dashModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.inputErr = nil
		m.input.Blur()
		return m, nil

	case tea.KeyEnter:
		root, err := validateRoot(m.input.Value())
		if err != nil {
			m.inputErr = err
			return m, nil
		}
		m.mode = modeNormal
		m.inputErr = nil
		m.input.Blur()
		m.pendingRoot = root
		m.scanErr = nil
		m.resetList()
		return m, tea.Batch(scanCmd(root), m.spin.Tick)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.inputErr = nil
	return m, cmd
}

// validateRoot turns the typed value into a usable scan root: it expands a
// leading ~, requires an absolute path, and checks it is an existing
// directory.
func validateRoot(value string) (string, error) {
	root := strings.TrimSpace(value)
	if root == "" {
		return "", fmt.Errorf("path is required")
	}

	if root == "~" || strings.HasPrefix(root, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve ~: %v", err)
		}
		root = filepath.Join(home, strings.TrimPrefix(root, "~"))
	}

	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("path must be absolute")
	}

	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("no such directory")
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory")
	}

	return filepath.Clean(root), nil
}

// Layout constants: two header lines (info + tabs), one footer line, and the
// two lines a panel border adds around its content.
const (
	dashChromeLines  = 3
	panelBorderLines = 2
	minPanelWidth    = 20
	inputPrompt      = " new root ▸ "
	// inputErrorReserve is the width kept free at the end of the input line
	// for the longest validation error ("✗ path must be absolute" plus its
	// separator). The input scrolls horizontally, so capping its window
	// loses nothing.
	inputErrorReserve = 34
	minInputWidth     = 20
)

// render draws the dashboard. It returns an empty string until the first
// WindowSizeMsg arrives, since there is no size to lay out against yet.
func (m dashModel) render() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	header := m.viewHeader() + "\n" + m.viewTabs()
	footer := m.viewFooter()

	if m.page != nil {
		page := panelFocusedStyle.Width(m.width - panelBorderLines).Height(m.bodyHeight()).
			MaxHeight(m.bodyHeight() + panelBorderLines).Render(m.viewPage())
		return lipgloss.JoinVertical(lipgloss.Left, header, page, footer)
	}

	leftWidth, rightWidth := m.panelWidths()
	bodyHeight := m.bodyHeight()

	// The inner widths subtract the panel's horizontal padding.
	left := m.panelStyleFor(panelPaths).Width(leftWidth).Height(bodyHeight).MaxHeight(bodyHeight + panelBorderLines).
		Render(m.viewList(leftWidth - 2))
	right := m.panelStyleFor(panelDetail).Width(rightWidth).Height(bodyHeight).MaxHeight(bodyHeight + panelBorderLines).
		Render(m.viewDetail(rightWidth - 2))

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// maxRootDisplay caps how much of the root path the header shows; longer
// paths keep their tail, which is the part that identifies them.
const maxRootDisplay = 48

// viewHeader renders the top bar: the wordmark, the version, the current
// root, and the scan-in-flight indicator. It is clipped to the terminal
// width so a long path can never wrap the layout.
func (m dashModel) viewHeader() string {
	header := " " + headerTitleStyle.Render("adev") + " " + headerVersionStyle.Render(m.version) +
		"  " + headerVersionStyle.Render("root:") + " " + displayRoot(m.root)

	if m.pendingRoot != "" {
		header += "  " + m.spin.View() + " " + headerVersionStyle.Render("scanning "+displayRoot(m.pendingRoot))
	}

	return lipgloss.NewStyle().MaxWidth(m.width).Render(header)
}

// viewTabs renders the view switcher line, the active view highlighted.
func (m dashModel) viewTabs() string {
	parts := make([]string, len(viewNames))
	for i, name := range viewNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		if dashView(i) == m.view {
			parts[i] = headerTitleStyle.Render(label)
		} else {
			parts[i] = footerStyle.Render(label)
		}
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(" " + strings.Join(parts, footerStyle.Render("  ")))
}

// displayRoot abbreviates a root path for the header: the home prefix
// becomes ~, and anything still longer than maxRootDisplay keeps only its
// tail.
func displayRoot(path string) string {
	p := abbreviateHome(path)
	runes := []rune(p)
	if len(runes) <= maxRootDisplay {
		return p
	}
	return "…" + string(runes[len(runes)-maxRootDisplay+1:])
}

// viewFooter renders the key hints, or whichever prompt/status owns the
// footer line: the root input, the delete confirm, the running-action
// spinner, or the last action's result.
func (m dashModel) viewFooter() string {
	switch {
	case m.mode == modeInput:
		line := inputPromptStyle.Render(inputPrompt) + m.input.View()
		if m.inputErr != nil {
			line += "  " + errorTextStyle.Render("✗ "+m.inputErr.Error())
		}
		return line

	case m.mode == modeConfirm && m.pending != nil:
		if m.pending.confirmName != "" {
			line := errorTextStyle.Render(" type "+m.pending.confirmName+" to delete it all ▸ ") + m.input.View()
			if m.inputErr != nil {
				line += "  " + errorTextStyle.Render("✗ "+m.inputErr.Error())
			}
			return line
		}
		return errorTextStyle.Render(" "+m.pending.question()) + footerStyle.Render(" y/n")

	case m.busy:
		return " " + m.spin.View() + footerStyle.Render(" working...")

	case m.status != "":
		return " " + m.status
	}

	if m.page != nil {
		return footerStyle.Render(" esc: back · j/k: scroll · q: quit")
	}
	return footerStyle.Render(" enter: open · d: delete · t: toggle · 1-5: view · o: root · j/k: move · q: quit")
}

// viewList renders the left panel: the rows of the active view, one line per
// entry, scrolled to keep the selection visible, with the lazygit-style
// position counter next to the title.
func (m dashModel) viewList(inner int) string {
	var b strings.Builder

	total := m.listLen()
	title := panelTitleStyle.Render(m.listTitle())
	if total > 0 {
		title += " " + itemMutedStyle.Render(fmt.Sprintf("%d/%d", m.selected+1, total))
	}
	b.WriteString(title + "\n\n")

	switch {
	case m.scanErr != nil:
		b.WriteString(errorTextStyle.Render("scan failed: " + m.scanErr.Error()))

	case total == 0 && m.pendingRoot != "":
		b.WriteString(itemMutedStyle.Render("scanning..."))

	case total == 0 && m.view == viewDoctorTab:
		b.WriteString(itemSelectedStyle.Render("✓ no problems found"))

	case total == 0:
		b.WriteString(itemMutedStyle.Render("nothing found\nunder " + abbreviateHome(m.root)))

	default:
		end := m.pathsOffset + m.listCapacity()
		if end > total {
			end = total
		}
		for i := m.pathsOffset; i < end; i++ {
			prefix, style := "  ", itemStyle
			if i == m.selected {
				prefix, style = "▸ ", itemSelectedStyle
			}
			b.WriteString(prefix + m.rowLabel(i, inner-lipgloss.Width(prefix), style) + "\n")
		}
	}

	return b.String()
}

// listTitle names the active browse list.
func (m dashModel) listTitle() string {
	if m.view == viewPathsTab && m.drilled && m.drillParent < len(m.dirs) {
		return truncateTail(m.displayPath(m.dirs[m.drillParent].Path), 24)
	}
	name := viewNames[m.view]
	return strings.ToUpper(name[:1]) + name[1:]
}

// rowLabel renders row i of the active view, fitted to budget cells.
func (m dashModel) rowLabel(i, budget int, style lipgloss.Style) string {
	switch m.view {
	case viewPathsTab:
		if m.drilled {
			return m.drillRowLabel(i, budget, style)
		}
		dir := m.dirs[i]
		counts := fmt.Sprintf("%ds %dp %dm",
			len(dir.Skills), len(dir.Plugins), len(dir.Marketplaces))
		path := truncateTail(m.displayPath(dir.Path), budget-lipgloss.Width(counts)-1)
		return style.Render(path) + " " + itemMutedStyle.Render(counts)

	case viewSkillsTab:
		g := m.skills[i]
		return groupRow(g.Name, len(g.Locations), budget, style)

	case viewPluginsTab:
		g := m.plugins[i]
		return groupRow(g.Key, len(g.Locations), budget, style)

	case viewMarketsTab:
		g := m.markets[i]
		return groupRow(g.Name, len(g.Locations), budget, style)

	default:
		f := m.findings[i]
		tag, tagStyle := "E", itemErrorStyle
		if f.Severity == doctor.Warning {
			tag, tagStyle = "W", itemWarnStyle
		}
		return tagStyle.Render(tag+" ") + style.Render(truncateHead(f.Message, budget-2))
	}
}

// drillRowLabel renders one artifact row of the drilled config dir, tagged
// by kind.
func (m dashModel) drillRowLabel(i, budget int, style lipgloss.Style) string {
	refs := m.drillRefs()
	if i >= len(refs) {
		return ""
	}
	dir := m.dirs[m.drillParent]
	ref := refs[i]

	var tag, name string
	switch ref.kind {
	case refSkill:
		tag, name = "s", dir.Skills[ref.index].Name
	case refPlugin:
		p := dir.Plugins[ref.index]
		tag, name = "p", p.Name
		if p.Marketplace != "" {
			name += "@" + p.Marketplace
		}
	default:
		tag, name = "m", dir.Marketplaces[ref.index].Name
	}

	return itemMutedStyle.Render(tag+" ") + style.Render(truncateTail(name, budget-2))
}

// groupRow renders "name ×N" fitted to budget cells.
func groupRow(name string, locations, budget int, style lipgloss.Style) string {
	count := fmt.Sprintf("×%d", locations)
	return style.Render(truncateTail(name, budget-lipgloss.Width(count)-1)) + " " + itemMutedStyle.Render(count)
}

// viewDetail renders the right panel: the preview of the current selection,
// windowed by the detail scroll position. When the content overflows, the
// title shows how far down the window is.
func (m dashModel) viewDetail(inner int) string {
	lines := m.detailLines(inner)
	capacity := m.listCapacity()

	offset := m.detailOffset
	if offset > len(lines)-capacity {
		offset = len(lines) - capacity
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + capacity
	if end > len(lines) {
		end = len(lines)
	}

	title := panelTitleStyle.Render("Detail")
	if len(lines) > capacity {
		title += " " + itemMutedStyle.Render(fmt.Sprintf("%d%%", end*100/len(lines)))
	}

	return title + "\n\n" + strings.Join(lines[offset:end], "\n")
}

// viewPage renders the full-screen page content, windowed by its scroll.
func (m dashModel) viewPage() string {
	lines := wrapLines(m.page.content, m.pageInnerWidth())
	capacity := m.listCapacity()

	offset := m.page.offset
	if offset > len(lines)-capacity {
		offset = len(lines) - capacity
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + capacity
	if end > len(lines) {
		end = len(lines)
	}

	title := panelTitleStyle.Render(m.page.title)
	if len(lines) > capacity {
		title += " " + itemMutedStyle.Render(fmt.Sprintf("%d%%", end*100/len(lines)))
	}

	return title + "\n\n" + strings.Join(lines[offset:end], "\n")
}

// detailLines builds the preview of the current selection, wrapped at the
// panel's inner width so windowing operates on real screen lines.
func (m dashModel) detailLines(inner int) []string {
	content := itemMutedStyle.Render("nothing selected")

	if m.listLen() > 0 && m.selected < m.listLen() {
		switch m.view {
		case viewPathsTab:
			if m.drilled {
				_, content = m.artifactPage(m.drillRefs()[m.selected])
			} else {
				content = configDirPreview(m.dirs[m.selected])
			}
		case viewSkillsTab:
			content = skillGroupPreview(m.skills[m.selected])
		case viewPluginsTab:
			content = pluginGroupPreview(m.plugins[m.selected])
		case viewMarketsTab:
			content = marketplaceGroupPreview(m.markets[m.selected])
		default:
			content = findingPreview(m.findings[m.selected])
		}
	}

	return wrapLines(content, inner)
}

// wrapLines hard-wraps styled content at width and splits it into lines.
func wrapLines(content string, width int) []string {
	wrapped := lipgloss.NewStyle().Width(width).Render(strings.TrimRight(content, "\n"))
	return strings.Split(wrapped, "\n")
}

// truncateTail fits s into max cells, keeping the tail: for a path that is
// the identifying part.
func truncateTail(s string, max int) string {
	if max < 1 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return "…" + string(runes[len(runes)-max+1:])
}

// truncateHead fits s into max cells, keeping the head: for a finding
// message that is the identifying part.
func truncateHead(s string, max int) string {
	if max < 1 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// displayPath shows a config dir compactly: relative to the scan root when
// possible, with the home abbreviation as a fallback.
func (m dashModel) displayPath(path string) string {
	if rel, err := filepath.Rel(m.root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return abbreviateHome(path)
}

// abbreviateHome replaces the user's home prefix with ~ for display.
func abbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// panelStyleFor returns the focused or blurred panel frame for p.
func (m dashModel) panelStyleFor(p dashPanel) lipgloss.Style {
	if m.focus == p {
		return panelFocusedStyle
	}
	return panelStyle
}
