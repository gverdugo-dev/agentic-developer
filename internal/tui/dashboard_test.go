package tui

import (
	"agentic-developer/internal/clean"
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/doctor"
	"agentic-developer/internal/manage"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// key builds the KeyMsg bubbletea delivers for typed characters.
func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// discoveryDir builds a minimal ConfigDir fixture.
func discoveryDir(path string) discovery.ConfigDir {
	return discovery.ConfigDir{Path: path}
}

// TestRootInputFlow drives the dashboard through the root-change flow: "o"
// opens the input, an invalid path errors in place, a valid one launches a
// scan and returns to normal mode.
func TestRootInputFlow(t *testing.T) {
	m := newDash("test", t.TempDir())
	m.setSize(100, 30)
	m.pendingRoot = "" // pretend the initial scan already finished

	// "o" opens the input pre-filled with the current root.
	m, _ = m.Update(key("o"))
	if m.mode != modeInput {
		t.Fatalf("after 'o': mode = %v, want modeInput", m.mode)
	}
	if m.input.Value() != m.root {
		t.Fatalf("input not pre-filled: %q, want %q", m.input.Value(), m.root)
	}
	if !strings.Contains(m.viewFooter(), "new root") {
		t.Fatalf("footer does not show the input line: %q", m.viewFooter())
	}

	// A relative path is rejected on enter and the input stays open.
	m.input.SetValue("relative/path")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.inputErr == nil || !strings.Contains(m.inputErr.Error(), "absolute") {
		t.Fatalf("relative path: inputErr = %v, want absolute-path error", m.inputErr)
	}
	if m.mode != modeInput {
		t.Fatalf("after invalid enter: mode = %v, want modeInput", m.mode)
	}

	// A valid absolute path closes the input and launches the scan.
	valid := t.TempDir()
	m.input.SetValue(valid)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal {
		t.Fatalf("after valid enter: mode = %v, want modeNormal", m.mode)
	}
	if m.pendingRoot != valid {
		t.Fatalf("pendingRoot = %q, want %q", m.pendingRoot, valid)
	}
	if cmd == nil {
		t.Fatal("valid enter returned no scan command")
	}

	// esc cancels the input.
	m, _ = m.Update(key("o"))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Fatalf("after esc: mode = %v, want modeNormal", m.mode)
	}
}

// TestPathsScrollFollowsSelection verifies the paths window scrolls to keep
// the selected row visible in both directions.
func TestPathsScrollFollowsSelection(t *testing.T) {
	m := newDash("test", "/root")
	m.pendingRoot = ""
	m.setSize(100, 12) // listCapacity = 12 - 2 - 2 - 2 = 6
	for i := 0; i < 20; i++ {
		m.dirs = append(m.dirs, discoveryDir("/root/p"+string(rune('a'+i))))
	}

	capacity := m.listCapacity()

	// Walk down past the window: the offset must follow.
	for i := 0; i < 10; i++ {
		m, _ = m.Update(key("j"))
	}
	if m.selected != 10 {
		t.Fatalf("selected = %d, want 10", m.selected)
	}
	if m.selected < m.pathsOffset || m.selected >= m.pathsOffset+capacity {
		t.Fatalf("selected %d outside window [%d, %d)", m.selected, m.pathsOffset, m.pathsOffset+capacity)
	}

	// Walk back up: the offset must follow again.
	for i := 0; i < 10; i++ {
		m, _ = m.Update(key("k"))
	}
	if m.selected != 0 || m.pathsOffset != 0 {
		t.Fatalf("after walking up: selected = %d, offset = %d, want 0, 0", m.selected, m.pathsOffset)
	}
}

// TestDetailScrollClamped verifies detail scrolling never goes past the
// content and resets when the selection changes.
func TestDetailScrollClamped(t *testing.T) {
	m := newDash("test", "/root")
	m.pendingRoot = ""
	m.setSize(100, 12)
	long := discoveryDir("/root/a")
	for i := 0; i < 30; i++ {
		long.Skills = append(long.Skills, discovery.Skill{Name: "skill"})
	}
	m.dirs = append(m.dirs, long, discoveryDir("/root/b"))

	m.focus = panelDetail
	for i := 0; i < 100; i++ {
		m, _ = m.Update(key("j"))
	}
	if m.detailOffset <= 0 {
		t.Fatal("detail did not scroll")
	}
	_, right := m.panelWidths()
	maxOffset := len(m.detailLines(right-2)) - m.listCapacity()
	if m.detailOffset > maxOffset {
		t.Fatalf("detailOffset = %d beyond max %d", m.detailOffset, maxOffset)
	}

	// Changing the selection resets the detail scroll.
	m.focus = panelPaths
	m, _ = m.Update(key("j"))
	if m.detailOffset != 0 {
		t.Fatalf("detailOffset = %d after selection change, want 0", m.detailOffset)
	}
}

// TestStaleScanDropped verifies a result for a root that is no longer pending
// is ignored.
func TestStaleScanDropped(t *testing.T) {
	m := newDash("test", "/a")
	m.pendingRoot = "/b"

	m, _ = m.Update(scanResultMsg{root: "/a"})
	if m.pendingRoot != "/b" {
		t.Fatal("stale result cleared pendingRoot")
	}

	m, _ = m.Update(scanResultMsg{root: "/b"})
	if m.pendingRoot != "" {
		t.Fatal("matching result did not clear pendingRoot")
	}
	if m.root != "/b" {
		t.Fatalf("root = %q, want /b", m.root)
	}
}

// TestViewSwitchDrillAndPage drives the full navigation: switching to the
// skills view, opening a group page, and drilling into a config dir from the
// paths view.
func TestViewSwitchDrillAndPage(t *testing.T) {
	m := newDash("test", "/root")
	m.setSize(100, 30)
	m, _ = m.Update(scanResultMsg{root: "/root", dirs: []discovery.ConfigDir{
		{Path: "/root/a/.claude", Skills: []discovery.Skill{{Name: "x", Description: "does x"}, {Name: "y"}}},
		{Path: "/root/b/.codex", Skills: []discovery.Skill{{Name: "x"}}},
	}})

	// Switch to the skills view: two groups (x in 2 places, y in 1).
	m, _ = m.Update(key("2"))
	if m.view != viewSkillsTab || m.listLen() != 2 {
		t.Fatalf("skills view: view=%v listLen=%d, want viewSkillsTab with 2", m.view, m.listLen())
	}

	// Enter opens the group page; esc closes it.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.page == nil || !strings.Contains(m.page.title, "x") {
		t.Fatalf("page = %+v, want skill x page", m.page)
	}
	if !strings.Contains(m.page.content, "2 location(s)") {
		t.Fatalf("page content misses location count: %q", m.page.content)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.page != nil {
		t.Fatal("esc did not close the page")
	}

	// Back to paths, drill into the second config dir, open an artifact.
	m, _ = m.Update(key("1"))
	m, _ = m.Update(key("j"))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.drilled || m.drillParent != 1 || m.listLen() != 1 {
		t.Fatalf("drill: drilled=%v parent=%d listLen=%d", m.drilled, m.drillParent, m.listLen())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.page == nil || m.page.title != "skill: x" {
		t.Fatalf("artifact page = %+v, want skill: x", m.page)
	}

	// esc unwinds: page, then drill (restoring the parent selection).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.page != nil || !m.drilled {
		t.Fatal("first esc should only close the page")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.drilled || m.selected != 1 {
		t.Fatalf("second esc should undrill and restore selection: drilled=%v selected=%d", m.drilled, m.selected)
	}
}

// TestGroupRowDriftBadge verifies duplicated group rows carry the content
// badge: "=" for identical copies, "≠" for drifted ones, none otherwise.
func TestGroupRowDriftBadge(t *testing.T) {
	cases := []struct {
		name      string
		locations int
		drift     discovery.DriftState
		want      string
		absent    []string
	}{
		{name: "identical", locations: 2, drift: discovery.DriftIdentical, want: "=", absent: []string{"≠"}},
		{name: "drifted", locations: 2, drift: discovery.DriftDrifted, want: "≠", absent: []string{"="}},
		{name: "single", locations: 1, drift: discovery.DriftSingle, want: "×1", absent: []string{"=", "≠"}},
		{name: "unknown", locations: 2, drift: discovery.DriftUnknown, want: "×2", absent: []string{"=", "≠"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := groupRow("my-skill", tc.locations, tc.drift, 40, itemStyle)
			if !strings.Contains(row, tc.want) {
				t.Fatalf("row %q misses %q", row, tc.want)
			}
			for _, s := range tc.absent {
				if strings.Contains(row, s) {
					t.Fatalf("row %q should not contain %q", row, s)
				}
			}
		})
	}
}

// TestGroupPreviewHashState verifies the group detail shows the drift
// summary and the per-location hash state.
func TestGroupPreviewHashState(t *testing.T) {
	drifted := discovery.SkillGroup{
		Name:  "x",
		Drift: discovery.DriftDrifted,
		Locations: []discovery.Located[discovery.Skill]{
			{ConfigDir: "/a/.claude", Hash: "aaaa1111bbbb"},
			{ConfigDir: "/b/.codex", Hash: "cccc2222dddd", DiffCount: 3},
		},
	}
	content := skillGroupPreview(drifted)
	if !strings.Contains(content, "content drifted") {
		t.Fatalf("preview misses the drift summary: %q", content)
	}
	if !strings.Contains(content, "aaaa1111") || !strings.Contains(content, "cccc2222") {
		t.Fatalf("preview misses the short hashes: %q", content)
	}
	if !strings.Contains(content, "3 file(s) differ") {
		t.Fatalf("preview misses the differing-file count: %q", content)
	}

	identical := drifted
	identical.Drift = discovery.DriftIdentical
	identical.Locations = []discovery.Located[discovery.Skill]{
		{ConfigDir: "/a/.claude", Hash: "aaaa1111bbbb"},
		{ConfigDir: "/b/.codex", Hash: "aaaa1111bbbb"},
	}
	content = skillGroupPreview(identical)
	if !strings.Contains(content, "identical in every location") {
		t.Fatalf("preview misses the identical summary: %q", content)
	}
	if strings.Contains(content, "differ") {
		t.Fatalf("identical preview should not report differing files: %q", content)
	}
}

// TestDoctorView drives the doctor view: "5" switches to it, the rows list
// the findings, enter opens a detail page with the fix hint, and "d" is
// refused (findings are informational).
func TestDoctorView(t *testing.T) {
	m := newDash("test", "/root")
	m.setSize(100, 30)
	m, _ = m.Update(scanResultMsg{root: "/root",
		dirs: []discovery.ConfigDir{{Path: "/root/.claude"}},
		findings: []doctor.Finding{
			{Severity: doctor.Error, Check: "skill-md-missing", Path: "/root/.claude/skills/broken",
				Message: `skill "broken" has no SKILL.md, so it can never load`, FixHint: "create SKILL.md"},
			{Severity: doctor.Warning, Check: "skill-md-too-long", Path: "/root/.claude/skills/long/SKILL.md",
				Message: `SKILL.md of "long" is 250 lines long`, FixHint: "move knowledge into references/"},
		},
	})

	// "5" switches to the doctor view listing both findings.
	m, _ = m.Update(key("5"))
	if m.view != viewDoctorTab || m.listLen() != 2 {
		t.Fatalf("doctor view: view=%v listLen=%d, want viewDoctorTab with 2", m.view, m.listLen())
	}
	if !strings.Contains(m.viewList(80), "has no SKILL.md") {
		t.Fatalf("list misses the finding message: %q", m.viewList(80))
	}

	// The preview shows the fix hint; enter opens the full page.
	if preview := strings.Join(m.detailLines(60), "\n"); !strings.Contains(preview, "create SKILL.md") {
		t.Fatalf("preview misses the fix hint: %q", preview)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.page == nil || m.page.title != "finding: skill-md-missing" {
		t.Fatalf("page = %+v, want the finding page", m.page)
	}
	if !strings.Contains(m.page.content, "create SKILL.md") {
		t.Fatalf("page content misses the fix hint: %q", m.page.content)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// "d" does not open a confirm prompt: findings cannot be deleted.
	m, _ = m.Update(key("d"))
	if m.mode != modeNormal || m.pending != nil {
		t.Fatalf("d on a finding opened a confirm: mode=%v pending=%+v", m.mode, m.pending)
	}
	if m.status == "" {
		t.Fatal("d on a finding left no status explanation")
	}

	// The second finding is a warning and previews as one.
	m, _ = m.Update(key("j"))
	if preview := strings.Join(m.detailLines(60), "\n"); !strings.Contains(preview, "warning") {
		t.Fatalf("warning preview misses its severity: %q", preview)
	}

	// A rescan with no findings shows the healthy state.
	m.pendingRoot = "/root"
	m, _ = m.Update(scanResultMsg{root: "/root", dirs: []discovery.ConfigDir{{Path: "/root/.claude"}}})
	if m.listLen() != 0 || !strings.Contains(m.viewList(80), "no problems found") {
		t.Fatalf("healthy doctor list = %q, want 'no problems found'", m.viewList(80))
	}
}

// TestDeleteConfirmFlow drives "d" end to end on a real temp skill: confirm
// prompt, y, action runs, result triggers a rescan.
func TestDeleteConfirmFlow(t *testing.T) {
	base := t.TempDir()
	skillPath := base + "/.claude/skills/doomed"
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatal(err)
	}

	m := newDash("test", base)
	m.setSize(100, 30)
	m, _ = m.Update(scanResultMsg{root: base, dirs: []discovery.ConfigDir{
		{Path: base + "/.claude", Skills: []discovery.Skill{{Name: "doomed", Path: skillPath}}},
	}})

	// Drill in and request the delete.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(key("d"))
	if m.mode != modeConfirm || m.pending == nil || m.pending.kind != actionDelete {
		t.Fatalf("after d: mode=%v pending=%+v", m.mode, m.pending)
	}

	// "n" cancels without touching anything.
	m, _ = m.Update(key("n"))
	if m.mode != modeNormal || m.pending != nil {
		t.Fatal("n did not cancel")
	}
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatal("cancel still deleted the skill")
	}

	// d again, y confirms: the returned command performs the delete.
	m, _ = m.Update(key("d"))
	m, cmd := m.Update(key("y"))
	if !m.busy || cmd == nil {
		t.Fatalf("y did not start the action: busy=%v", m.busy)
	}
	result := findActionResult(t, cmd())
	if result.err != nil {
		t.Fatalf("delete failed: %v", result.err)
	}
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Fatal("the skill still exists after confirm")
	}

	// The result message clears busy and schedules the refresh scan.
	m, _ = m.Update(result)
	if m.busy || m.pendingRoot != base {
		t.Fatalf("after result: busy=%v pendingRoot=%q", m.busy, m.pendingRoot)
	}
}

// TestConfigDirDeleteNeedsTypedName verifies deleting a whole config dir
// requires typing its displayed name.
func TestConfigDirDeleteNeedsTypedName(t *testing.T) {
	base := t.TempDir()
	claude := base + "/.claude"
	if err := os.MkdirAll(claude+"/skills", 0o755); err != nil {
		t.Fatal(err)
	}

	m := newDash("test", base)
	m.setSize(100, 30)
	m, _ = m.Update(scanResultMsg{root: base, dirs: []discovery.ConfigDir{{Path: claude}}})

	m, _ = m.Update(key("d"))
	if m.mode != modeConfirm || m.pending == nil || m.pending.confirmName == "" {
		t.Fatalf("config dir delete lacks the name brake: %+v", m.pending)
	}

	// A wrong name is rejected.
	m.input.SetValue("nope")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.inputErr == nil || m.pending == nil {
		t.Fatal("wrong name was accepted")
	}

	// The right name launches the delete.
	m.input.SetValue(m.pending.confirmName)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.busy || cmd == nil {
		t.Fatal("right name did not start the action")
	}
	result := findActionResult(t, cmd())
	if result.err != nil {
		t.Fatalf("delete failed: %v", result.err)
	}
	if _, err := os.Stat(claude); !os.IsNotExist(err) {
		t.Fatal("the config dir still exists")
	}
}

// TestToggleUsesClaudeCLI verifies "t" on a registry plugin shells out with
// the right arguments, via the stubbed executor.
func TestToggleUsesClaudeCLI(t *testing.T) {
	var got [][]string
	orig := manage.Exec
	manage.Exec = func(args ...string) (string, error) {
		got = append(got, args)
		return "", nil
	}
	defer func() { manage.Exec = orig }()

	m := newDash("test", "/root")
	m.setSize(100, 30)
	m, _ = m.Update(scanResultMsg{root: "/root", dirs: []discovery.ConfigDir{
		{Path: "/root/.claude", Plugins: []discovery.Plugin{
			{Name: "ai", Marketplace: "mkt", Enabled: true},
		}},
	}})

	// Drill to the plugin row and toggle it: enabled -> disable.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := m.Update(key("t"))
	if !m.busy || cmd == nil {
		t.Fatalf("t did not start the toggle: busy=%v", m.busy)
	}
	result := findActionResult(t, cmd())
	if result.err != nil {
		t.Fatalf("toggle failed: %v", result.err)
	}
	if len(got) != 1 || got[0][0] != "plugin" || got[0][1] != "disable" || got[0][2] != "ai@mkt" {
		t.Fatalf("claude called with %v, want plugin disable ai@mkt", got)
	}
}

// marketsFixture builds a dashboard sitting on the marketplaces view with
// one registered marketplace offering two plugins, one of them installed.
func marketsFixture(t *testing.T) dashModel {
	t.Helper()
	m := newDash("test", "/root")
	m.setSize(100, 30)
	m, _ = m.Update(scanResultMsg{root: "/root", dirs: []discovery.ConfigDir{
		{
			Path:    "/root/.claude",
			Plugins: []discovery.Plugin{{Name: "ai", Marketplace: "mkt", Enabled: true}},
			Marketplaces: []discovery.Marketplace{
				{Name: "mkt", Source: "github o/r", PluginNames: []string{"ai", "research"}},
			},
		},
	}})
	m, _ = m.Update(key("4"))
	if m.view != viewMarketsTab || m.listLen() != 1 {
		t.Fatalf("marketplaces view: view=%v listLen=%d", m.view, m.listLen())
	}
	return m
}

// TestInstallFromMarketplaceCatalog drives "i" end to end: it drills into
// the marketplace catalog, marks the installed entry, and installs the
// selected one through the claude CLI.
func TestInstallFromMarketplaceCatalog(t *testing.T) {
	var got [][]string
	orig := manage.Exec
	manage.Exec = func(args ...string) (string, error) {
		got = append(got, args)
		return "", nil
	}
	defer func() { manage.Exec = orig }()

	m := marketsFixture(t)

	// First "i" drills into the catalog instead of installing anything.
	m, _ = m.Update(key("i"))
	if !m.marketDrilled || m.listLen() != 2 {
		t.Fatalf("after i: marketDrilled=%v listLen=%d, want drilled with 2 entries", m.marketDrilled, m.listLen())
	}
	if len(got) != 0 {
		t.Fatalf("drilling already called claude: %v", got)
	}

	// The already-installed entry is marked; the other is not.
	if !strings.Contains(m.catalogEntryPreview(0), "installed") {
		t.Fatalf("entry 0 preview misses the installed state: %q", m.catalogEntryPreview(0))
	}
	if !strings.Contains(m.catalogEntryPreview(1), "not installed") {
		t.Fatalf("entry 1 preview misses the not-installed state: %q", m.catalogEntryPreview(1))
	}

	// "i" on the second entry installs research@mkt.
	m, _ = m.Update(key("j"))
	m, cmd := m.Update(key("i"))
	if !m.busy || cmd == nil {
		t.Fatalf("i did not start the install: busy=%v", m.busy)
	}
	result := findActionResult(t, cmd())
	if result.err != nil {
		t.Fatalf("install failed: %v", result.err)
	}
	if len(got) != 1 || strings.Join(got[0], " ") != "plugin install research@mkt" {
		t.Fatalf("claude called with %v, want plugin install research@mkt", got)
	}

	// The result clears busy and schedules the refresh scan.
	m, _ = m.Update(result)
	if m.busy || m.pendingRoot != "/root" {
		t.Fatalf("after result: busy=%v pendingRoot=%q", m.busy, m.pendingRoot)
	}
}

// TestCatalogDrillNavigation verifies esc backs out of the catalog restoring
// the marketplace selection, and that "i" outside the marketplaces view only
// leaves a hint.
func TestCatalogDrillNavigation(t *testing.T) {
	m := marketsFixture(t)

	m, _ = m.Update(key("i"))
	if !m.marketDrilled {
		t.Fatal("i did not drill into the catalog")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.marketDrilled || m.selected != 0 {
		t.Fatalf("esc did not undrill: drilled=%v selected=%d", m.marketDrilled, m.selected)
	}

	// Outside the marketplaces view "i" is a no-op with a hint.
	m, _ = m.Update(key("1"))
	m, cmd := m.Update(key("i"))
	if cmd != nil || m.busy || m.status == "" {
		t.Fatalf("i outside marketplaces: cmd=%v busy=%v status=%q", cmd, m.busy, m.status)
	}
}

// TestAddMarketplaceInputFlow drives "a" end to end: the footer input opens,
// an empty source errors in place, and a real one runs the claude CLI add
// and triggers the rescan.
func TestAddMarketplaceInputFlow(t *testing.T) {
	var got [][]string
	orig := manage.Exec
	manage.Exec = func(args ...string) (string, error) {
		got = append(got, args)
		return "", nil
	}
	defer func() { manage.Exec = orig }()

	m := marketsFixture(t)

	// "a" opens the source input with its own prompt.
	m, _ = m.Update(key("a"))
	if m.mode != modeInput || m.inputFor != inputMarketSource {
		t.Fatalf("after a: mode=%v inputFor=%v", m.mode, m.inputFor)
	}
	if !strings.Contains(m.viewFooter(), "add marketplace") {
		t.Fatalf("footer does not show the source input: %q", m.viewFooter())
	}

	// An empty source is rejected in place.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.inputErr == nil || m.mode != modeInput {
		t.Fatalf("empty source accepted: err=%v mode=%v", m.inputErr, m.mode)
	}

	// esc cancels and restores the idle placeholder.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal || m.input.Placeholder != rootPlaceholder {
		t.Fatalf("esc left mode=%v placeholder=%q", m.mode, m.input.Placeholder)
	}

	// A typed source launches the add through the claude CLI.
	m, _ = m.Update(key("a"))
	m.input.SetValue("owner/repo")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.busy || cmd == nil {
		t.Fatalf("enter did not start the add: busy=%v", m.busy)
	}
	result := findActionResult(t, cmd())
	if result.err != nil {
		t.Fatalf("add failed: %v", result.err)
	}
	if len(got) != 1 || strings.Join(got[0], " ") != "plugin marketplace add owner/repo" {
		t.Fatalf("claude called with %v, want plugin marketplace add owner/repo", got)
	}
	m, _ = m.Update(result)
	if m.busy || m.pendingRoot != "/root" {
		t.Fatalf("after result: busy=%v pendingRoot=%q", m.busy, m.pendingRoot)
	}

	// "a" outside the marketplaces view only leaves a hint.
	m.pendingRoot = ""
	m, _ = m.Update(key("1"))
	m, _ = m.Update(key("a"))
	if m.mode != modeNormal || m.status == "" {
		t.Fatalf("a outside marketplaces: mode=%v status=%q", m.mode, m.status)
	}
}

// cleanFixture builds a dashboard sitting on the doctor view with one
// finding and two clean candidates: a disk orphan (under base/.claude so the
// guarded delete accepts it) and a registry uninstall.
func cleanFixture(t *testing.T, base string) dashModel {
	t.Helper()
	orphan := base + "/.claude/plugins/cache/mkt/orphan"
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}

	m := newDash("test", base)
	m.setSize(100, 30)
	m, _ = m.Update(scanResultMsg{root: base,
		dirs: []discovery.ConfigDir{{Path: base + "/.claude"}},
		findings: []doctor.Finding{
			{Severity: doctor.Warning, Check: "cache-orphan", Path: orphan,
				Message: `cached plugin "orphan@mkt" has no registry entry`},
		},
		candidates: []clean.Candidate{
			{Kind: clean.KindCacheOrphan, Path: orphan, Size: 2048,
				Reason: `cached plugin "orphan@mkt" has no registry entry`, Action: clean.ActionRemoveDir},
			{Kind: clean.KindBrokenArtifact, Path: base + "/.claude/plugins/cache/mkt/broken/1.0.0",
				Reason: "broken plugin", Action: clean.ActionUninstall, Arg: "broken@mkt"},
		},
	})
	m, _ = m.Update(key("5"))
	if m.view != viewDoctorTab {
		t.Fatalf("view = %v, want viewDoctorTab", m.view)
	}
	return m
}

// TestCleanListFromDoctorView drives "c": the doctor view flips to the
// candidate list, rows carry the reclaimable size, the preview explains the
// removal, and esc returns to the findings.
func TestCleanListFromDoctorView(t *testing.T) {
	m := cleanFixture(t, t.TempDir())

	// The findings list is what "5" shows; "c" flips to the candidates.
	if m.listLen() != 1 {
		t.Fatalf("findings listLen = %d, want 1", m.listLen())
	}
	m, _ = m.Update(key("c"))
	if !m.cleanDrilled || m.listLen() != 2 {
		t.Fatalf("after c: cleanDrilled=%v listLen=%d, want drilled with 2", m.cleanDrilled, m.listLen())
	}
	if !strings.Contains(m.viewList(80), "2.0 KB") {
		t.Fatalf("candidate row misses the size: %q", m.viewList(80))
	}
	if preview := strings.Join(m.detailLines(60), "\n"); !strings.Contains(preview, "press d to remove it") {
		t.Fatalf("candidate preview misses the removal hint: %q", preview)
	}

	// Enter opens the candidate page.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.page == nil || m.page.title != "candidate: "+clean.KindCacheOrphan {
		t.Fatalf("page = %+v, want the candidate page", m.page)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// The registry candidate previews its claude CLI removal.
	m, _ = m.Update(key("j"))
	if preview := strings.Join(m.detailLines(60), "\n"); !strings.Contains(preview, "plugin uninstall") {
		t.Fatalf("uninstall preview misses the claude command: %q", preview)
	}

	// esc returns to the findings list.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.cleanDrilled || m.listLen() != 1 {
		t.Fatalf("after esc: cleanDrilled=%v listLen=%d, want the findings back", m.cleanDrilled, m.listLen())
	}

	// "c" outside the doctor view only leaves a hint.
	m, _ = m.Update(key("1"))
	m, _ = m.Update(key("c"))
	if m.cleanDrilled || m.status == "" {
		t.Fatalf("c outside doctor: cleanDrilled=%v status=%q", m.cleanDrilled, m.status)
	}
}

// TestCleanRemoveConfirmFlow drives "d" on a disk candidate end to end:
// confirm prompt, n cancels, y removes the folder and schedules the rescan.
func TestCleanRemoveConfirmFlow(t *testing.T) {
	base := t.TempDir()
	m := cleanFixture(t, base)
	orphan := base + "/.claude/plugins/cache/mkt/orphan"

	// "d" on the findings list is still refused, pointing at "c".
	m, _ = m.Update(key("d"))
	if m.pending != nil || !strings.Contains(m.status, "c") {
		t.Fatalf("d on findings: pending=%+v status=%q", m.pending, m.status)
	}

	m, _ = m.Update(key("c"))
	m, _ = m.Update(key("d"))
	if m.mode != modeConfirm || m.pending == nil || m.pending.kind != actionDelete {
		t.Fatalf("after d: mode=%v pending=%+v", m.mode, m.pending)
	}

	// "n" cancels without touching the folder.
	m, _ = m.Update(key("n"))
	if m.mode != modeNormal || m.pending != nil {
		t.Fatal("n did not cancel")
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Fatal("cancel still removed the orphan")
	}

	// d then y removes it and schedules the refresh scan.
	m, _ = m.Update(key("d"))
	m, cmd := m.Update(key("y"))
	if !m.busy || cmd == nil {
		t.Fatalf("y did not start the removal: busy=%v", m.busy)
	}
	result := findActionResult(t, cmd())
	if result.err != nil {
		t.Fatalf("removal failed: %v", result.err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("the orphan still exists after confirm")
	}
	m, _ = m.Update(result)
	if m.busy || m.pendingRoot != base {
		t.Fatalf("after result: busy=%v pendingRoot=%q", m.busy, m.pendingRoot)
	}

	// The rescan keeps the clean list open so it can be emptied one by one.
	if !m.cleanDrilled {
		t.Fatal("the rescan left the clean list")
	}
}

// TestCleanRegistryCandidateUsesClaudeCLI verifies confirming a registry
// candidate shells out to the claude CLI instead of touching the disk.
func TestCleanRegistryCandidateUsesClaudeCLI(t *testing.T) {
	var got [][]string
	orig := manage.Exec
	manage.Exec = func(args ...string) (string, error) {
		got = append(got, args)
		return "", nil
	}
	defer func() { manage.Exec = orig }()

	m := cleanFixture(t, t.TempDir())
	m, _ = m.Update(key("c"))
	m, _ = m.Update(key("j")) // the uninstall candidate
	m, _ = m.Update(key("d"))
	if m.mode != modeConfirm || m.pending == nil || m.pending.kind != actionUninstall {
		t.Fatalf("after d: mode=%v pending=%+v", m.mode, m.pending)
	}

	m, cmd := m.Update(key("y"))
	if !m.busy || cmd == nil {
		t.Fatal("y did not start the uninstall")
	}
	result := findActionResult(t, cmd())
	if result.err != nil {
		t.Fatalf("uninstall failed: %v", result.err)
	}
	if len(got) != 1 || strings.Join(got[0], " ") != "plugin uninstall broken@mkt" {
		t.Fatalf("claude called with %v, want plugin uninstall broken@mkt", got)
	}
}

// TestCleanEmptyList verifies the drilled clean view shows its healthy state
// when there is nothing to remove.
func TestCleanEmptyList(t *testing.T) {
	m := newDash("test", "/root")
	m.setSize(100, 30)
	m, _ = m.Update(scanResultMsg{root: "/root", dirs: []discovery.ConfigDir{{Path: "/root/.claude"}}})

	m, _ = m.Update(key("5"))
	m, _ = m.Update(key("c"))
	if m.listLen() != 0 || !strings.Contains(m.viewList(80), "nothing to clean") {
		t.Fatalf("empty clean list = %q, want 'nothing to clean'", m.viewList(80))
	}
}

// findActionResult unwraps the tea.Msg of a (possibly batched) action
// command into its actionResultMsg.
func findActionResult(t *testing.T, msg tea.Msg) actionResultMsg {
	t.Helper()
	switch v := msg.(type) {
	case actionResultMsg:
		return v
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			if r, ok := c().(actionResultMsg); ok {
				return r
			}
		}
	}
	t.Fatalf("no actionResultMsg in %T", msg)
	return actionResultMsg{}
}
