package tui

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/registry"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stubRegistryClient is a canned registry.Client for the explore tests.
type stubRegistryClient struct {
	skills   []registry.Skill
	err      error
	fetched  registry.Fetched
	fetchErr error
}

func (s stubRegistryClient) Search(string) ([]registry.Skill, error) { return s.skills, s.err }
func (s stubRegistryClient) FetchSkill(registry.Skill) (registry.Fetched, error) {
	return s.fetched, s.fetchErr
}

// exploreDash builds a dashboard sitting on the explore view with a stub
// registry client.
func exploreDash(t *testing.T, client registry.Client) dashModel {
	t.Helper()
	m := newDash("test", t.TempDir())
	m.setSize(100, 30)
	m.pendingRoot = ""
	m.reg = client
	m, _ = m.Update(key("6"))
	if m.view != viewExploreTab {
		t.Fatalf("after '6': view = %v, want explore", m.view)
	}
	return m
}

// registryFixtureSkill lays a skill dir the stub's Fetched can point at and
// returns the Fetched.
func registryFixtureSkill(t *testing.T, name string) registry.Fetched {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "---\nname: " + name + "\ndescription: \"Registry fixture skill\"\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return registry.Fetched{Dir: dir, Root: root, Manifest: manifest}
}

// TestExploreSearchFlow drives "/" + query + enter through a stubbed search
// and checks the results land in the list.
func TestExploreSearchFlow(t *testing.T) {
	client := stubRegistryClient{skills: []registry.Skill{
		{ID: "acme/demo/demo-skill", SkillID: "demo-skill", Name: "demo-skill", Installs: 4242, Source: "acme/demo"},
	}}
	m := exploreDash(t, client)

	if !strings.Contains(m.viewList(60), "press / to search") {
		t.Fatalf("empty explore list misses the hint: %q", m.viewList(60))
	}

	m, _ = m.Update(key("/"))
	if m.mode != modeInput || m.inputFor != inputExploreQuery {
		t.Fatalf("after '/': mode = %v inputFor = %v", m.mode, m.inputFor)
	}
	if !strings.Contains(m.viewFooter(), "search skills.sh") {
		t.Fatalf("footer misses the search prompt: %q", m.viewFooter())
	}

	// An empty query is rejected in place.
	m.input.SetValue("  ")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.inputErr == nil {
		t.Fatal("empty query accepted")
	}

	m.input.SetValue("demo")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.busy || cmd == nil {
		t.Fatal("enter did not launch the search")
	}
	m, _ = m.Update(findExploreSearch(t, cmd()))
	if m.busy {
		t.Fatal("still busy after the search result")
	}
	if len(m.exploreResults) != 1 || m.exploreResults[0].Name != "demo-skill" {
		t.Fatalf("results = %+v", m.exploreResults)
	}
	if !strings.Contains(m.viewList(60), "demo-skill") || !strings.Contains(m.viewList(60), "4242") {
		t.Fatalf("list misses the result row: %q", m.viewList(60))
	}
}

// TestExploreSearchErrorDegrades checks an unreachable registry surfaces in
// the status line and leaves the view usable.
func TestExploreSearchErrorDegrades(t *testing.T) {
	m := exploreDash(t, stubRegistryClient{err: errors.New("registry unreachable: no such host")})

	m, _ = m.Update(key("/"))
	m.input.SetValue("demo")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(findExploreSearch(t, cmd()))

	if m.busy || m.mode != modeNormal {
		t.Fatalf("view not usable after the failure: busy=%v mode=%v", m.busy, m.mode)
	}
	if !strings.Contains(m.status, "search failed") {
		t.Fatalf("status = %q, want the search failure", m.status)
	}
	// The view still takes a new search.
	m, _ = m.Update(key("/"))
	if m.mode != modeInput {
		t.Fatal("cannot search again after a failure")
	}
}

// TestExploreInstallIsNeverBlind drives the full security path: "i" before
// the preview is refused; enter fetches and opens the SKILL.md page; "i"
// then opens the picker, whose enter still requires the y/N confirm; "y"
// finally installs onto disk through the T7 copier.
func TestExploreInstallIsNeverBlind(t *testing.T) {
	fetched := registryFixtureSkill(t, "demo-skill")
	client := stubRegistryClient{
		skills:  []registry.Skill{{ID: "acme/demo/demo-skill", SkillID: "demo-skill", Name: "demo-skill", Installs: 7, Source: "acme/demo"}},
		fetched: fetched,
	}
	m := exploreDash(t, client)

	// A discovered config dir gives the install a target.
	target := filepath.Join(t.TempDir(), ".claude")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	m.dirs = []discovery.ConfigDir{{Path: target}}
	m.exploreResults = client.skills

	// "i" without the preview is refused.
	m, _ = m.Update(key("i"))
	if m.mode != modeNormal || !strings.Contains(m.status, "read it first") {
		t.Fatalf("blind install not refused: mode=%v status=%q", m.mode, m.status)
	}

	// enter fetches and opens the SKILL.md page.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.busy || cmd == nil {
		t.Fatal("enter did not launch the fetch")
	}
	m, _ = m.Update(findExploreFetch(t, cmd()))
	if m.page == nil || !strings.Contains(m.page.content, "name: demo-skill") {
		t.Fatalf("no SKILL.md page after the fetch: %+v", m.page)
	}

	// Close the page; "i" now opens the picker.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m, _ = m.Update(key("i"))
	if m.mode != modePicker || m.picker == nil || !m.picker.fromRegistry {
		t.Fatalf("no registry picker after the preview: mode=%v picker=%+v", m.mode, m.picker)
	}
	if !strings.Contains(m.viewFooter(), "install demo-skill to") {
		t.Fatalf("picker footer = %q", m.viewFooter())
	}

	// enter on the target opens the confirm, it does not install yet.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeConfirm || m.pending == nil {
		t.Fatalf("no confirm after picking: mode=%v", m.mode)
	}
	if !strings.Contains(m.pending.question(), "acme/demo") {
		t.Fatalf("confirm question misses the source: %q", m.pending.question())
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "demo-skill")); !os.IsNotExist(err) {
		t.Fatal("the skill landed before the confirm")
	}

	// "n" cancels; nothing lands.
	m, _ = m.Update(key("n"))
	if m.mode != modeNormal || m.pending != nil {
		t.Fatal("n did not cancel the confirm")
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "demo-skill")); !os.IsNotExist(err) {
		t.Fatal("the skill landed after a cancel")
	}

	// Redo the pick and confirm with "y": now it installs.
	m, _ = m.Update(key("i"))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd = m.Update(key("y"))
	if !m.busy || cmd == nil {
		t.Fatal("y did not launch the install")
	}
	if result := findActionResult(t, cmd()); result.err != nil {
		t.Fatalf("install failed: %v", result.err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "demo-skill", "SKILL.md")); err != nil {
		t.Fatal("the skill did not land after the confirm")
	}
}

// TestExploreDeleteRefused checks "d" never acts on registry results.
func TestExploreDeleteRefused(t *testing.T) {
	m := exploreDash(t, stubRegistryClient{})
	m.exploreResults = []registry.Skill{{ID: "a/b/c", SkillID: "c", Name: "c", Source: "a/b"}}

	m = m.requestDelete()
	if m.pending != nil || !strings.Contains(m.status, "not deleted") {
		t.Fatalf("delete not refused: pending=%v status=%q", m.pending, m.status)
	}
}

// findExploreSearch unwraps a (possibly batched) command into its
// exploreSearchMsg.
func findExploreSearch(t *testing.T, msg tea.Msg) exploreSearchMsg {
	t.Helper()
	switch msg := msg.(type) {
	case exploreSearchMsg:
		return msg
	case tea.BatchMsg:
		for _, c := range msg {
			if c == nil {
				continue
			}
			if r, ok := c().(exploreSearchMsg); ok {
				return r
			}
		}
	}
	t.Fatalf("no exploreSearchMsg in %T", msg)
	return exploreSearchMsg{}
}

// findExploreFetch unwraps a (possibly batched) command into its
// exploreFetchMsg.
func findExploreFetch(t *testing.T, msg tea.Msg) exploreFetchMsg {
	t.Helper()
	switch msg := msg.(type) {
	case exploreFetchMsg:
		return msg
	case tea.BatchMsg:
		for _, c := range msg {
			if c == nil {
				continue
			}
			if r, ok := c().(exploreFetchMsg); ok {
				return r
			}
		}
	}
	t.Fatalf("no exploreFetchMsg in %T", msg)
	return exploreFetchMsg{}
}
