package adevfile

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/harness"
	"strings"
	"testing"
)

// stubExec replaces harness.ClaudeExec for the test's lifetime, recording
// every argument list and answering with output.
func stubExec(t *testing.T, output string) *[][]string {
	t.Helper()
	var got [][]string
	orig := harness.ClaudeExec
	harness.ClaudeExec = func(args ...string) (string, error) {
		got = append(got, args)
		return output, nil
	}
	t.Cleanup(func() { harness.ClaudeExec = orig })
	return &got
}

// planItem finds the plan item of one category and name, failing when it is
// absent.
func planItem(t *testing.T, plan Plan, category, name string) Item {
	t.Helper()
	for _, item := range plan.Items {
		if item.Category == category && item.Name == name {
			return item
		}
	}
	t.Fatalf("no %s %q in plan: %+v", category, name, plan.Items)
	return Item{}
}

// TestDiff verifies the full diff against a fixture reality: satisfied,
// missing (with the right action per category and harness) and extra items.
func TestDiff(t *testing.T) {
	home := fixtureHome(t) // ~/.claude: home-skill, tool@some-mkt, some-mkt
	mkdirs(t, home, ".claude/skills/unlisted-skill")
	root := t.TempDir()

	manifest := File{
		Version: CurrentVersion,
		Harnesses: map[string]Harness{
			"claude": {User: &Entry{
				Skills:  []string{"home-skill", "wanted-skill"},
				Plugins: []string{"tool@some-mkt", "extra@other-mkt", "bare-plugin"},
				Marketplaces: []Marketplace{
					{Name: "some-mkt", Source: "acme/some-mkt"},
					{Name: "new-mkt", Source: "acme/new-mkt"},
					{Name: "sourceless-mkt"},
				},
			}},
			"codex": {Project: &Entry{
				Skills:  []string{"codex-skill"},
				Plugins: []string{"impossible@mkt"},
			}},
		},
	}

	dirs, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Diff(manifest, dirs, root)
	if err != nil {
		t.Fatal(err)
	}

	assert := func(item Item, state, action string) {
		t.Helper()
		if item.State != state || item.Action != action {
			t.Fatalf("%s = %s/%s, want %s/%s", item.Label(), item.State, item.Action, state, action)
		}
	}

	assert(planItem(t, plan, CategorySkill, "home-skill"), StateSatisfied, ActionNone)
	assert(planItem(t, plan, CategorySkill, "wanted-skill"), StateMissing, ActionManual) // T7 seam not wired
	assert(planItem(t, plan, CategorySkill, "unlisted-skill"), StateExtra, ActionNone)

	assert(planItem(t, plan, CategoryPlugin, "tool@some-mkt"), StateSatisfied, ActionNone)
	assert(planItem(t, plan, CategoryPlugin, "extra@other-mkt"), StateMissing, ActionInstall)
	assert(planItem(t, plan, CategoryPlugin, "bare-plugin"), StateMissing, ActionManual)

	assert(planItem(t, plan, CategoryMarketplace, "some-mkt"), StateSatisfied, ActionNone)
	newMkt := planItem(t, plan, CategoryMarketplace, "new-mkt")
	assert(newMkt, StateMissing, ActionAdd)
	if newMkt.Source != "acme/new-mkt" {
		t.Fatalf("new-mkt source = %q", newMkt.Source)
	}
	assert(planItem(t, plan, CategoryMarketplace, "sourceless-mkt"), StateMissing, ActionManual)

	// Codex has no CLI owning a registry, so even a keyed plugin is manual.
	assert(planItem(t, plan, CategorySkill, "codex-skill"), StateMissing, ActionManual)
	assert(planItem(t, plan, CategoryPlugin, "impossible@mkt"), StateMissing, ActionManual)

	if plan.InSync() {
		t.Fatal("plan reports in sync with missing items")
	}
	if plan.Missing() != 7 || plan.Extra() != 1 {
		t.Fatalf("missing/extra = %d/%d, want 7/1", plan.Missing(), plan.Extra())
	}
}

// TestDiffLeavesUndeclaredScopesAlone verifies a scope the manifest does not
// declare produces no items at all, even when its config dir has content.
func TestDiffLeavesUndeclaredScopesAlone(t *testing.T) {
	fixtureHome(t)
	root := t.TempDir()

	manifest := File{Version: CurrentVersion, Harnesses: map[string]Harness{
		"codex": {Project: &Entry{Skills: []string{"x"}}},
	}}

	dirs, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Diff(manifest, dirs, root)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range plan.Items {
		if item.Harness == "claude" {
			t.Fatalf("undeclared claude scope produced item %+v", item)
		}
	}
}

// TestApplyDelegates verifies Apply builds the right claude CLI invocation
// for installable items and refuses everything else.
func TestApplyDelegates(t *testing.T) {
	got := stubExec(t, "done")

	out, err := Apply(Item{
		Harness: "claude", Scope: ScopeUser,
		Category: CategoryPlugin, Name: "tool@mkt",
		State: StateMissing, Action: ActionInstall,
	})
	if err != nil || out != "done" {
		t.Fatalf("plugin install: out=%q err=%v", out, err)
	}

	if _, err := Apply(Item{
		Harness: "claude", Scope: ScopeUser,
		Category: CategoryMarketplace, Name: "mkt", Source: "acme/mkt",
		State: StateMissing, Action: ActionAdd,
	}); err != nil {
		t.Fatalf("marketplace add: %v", err)
	}

	want := []string{"plugin install tool@mkt", "plugin marketplace add acme/mkt"}
	if len(*got) != len(want) {
		t.Fatalf("claude called %d times, want %d: %v", len(*got), len(want), *got)
	}
	for i := range want {
		if strings.Join((*got)[i], " ") != want[i] {
			t.Fatalf("call %d = %v, want %q", i, (*got)[i], want[i])
		}
	}

	// Manual, satisfied and non-operable items never reach the CLI.
	refused := []Item{
		{Harness: "claude", Category: CategorySkill, Name: "s", State: StateMissing, Action: ActionManual},
		{Harness: "claude", Category: CategoryPlugin, Name: "p@m", State: StateSatisfied, Action: ActionNone},
		{Harness: "codex", Category: CategoryPlugin, Name: "p@m", State: StateMissing, Action: ActionInstall},
	}
	for _, item := range refused {
		if _, err := Apply(item); err == nil {
			t.Fatalf("Apply accepted %+v", item)
		}
	}
	if len(*got) != len(want) {
		t.Fatalf("a refused item reached the claude CLI: %v", *got)
	}
}

// TestInstallSkillSeam verifies that wiring the T7 seam flips missing skills
// from manual to install, and that Apply routes through it.
func TestInstallSkillSeam(t *testing.T) {
	var calls []string
	InstallSkill = func(name, harnessLabel, scope string) (string, error) {
		calls = append(calls, name+" into "+harnessLabel+"/"+scope)
		return "copied", nil
	}
	t.Cleanup(func() { InstallSkill = nil })

	fixtureHome(t)
	root := t.TempDir()
	manifest := File{Version: CurrentVersion, Harnesses: map[string]Harness{
		"claude": {User: &Entry{Skills: []string{"wanted-skill"}}},
	}}

	dirs, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Diff(manifest, dirs, root)
	if err != nil {
		t.Fatal(err)
	}

	item := planItem(t, plan, CategorySkill, "wanted-skill")
	if item.Action != ActionInstall {
		t.Fatalf("with the seam wired, action = %s, want install", item.Action)
	}

	out, err := Apply(item)
	if err != nil || out != "copied" {
		t.Fatalf("Apply through the seam: out=%q err=%v", out, err)
	}
	if len(calls) != 1 || calls[0] != "wanted-skill into claude/user" {
		t.Fatalf("seam calls = %v", calls)
	}
}
