package adevfile

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/harness"
	"agentic-developer/internal/scaffolding"
	"fmt"
	"sort"
	"strings"
)

// This file is the sync engine: Diff compares a manifest against the
// discovered reality and produces a plan, and Apply executes one plan item
// through the harness's own operation executor (for Claude, the claude CLI:
// adev never writes its registry files by hand). Deleting is deliberately
// not here: extras are reported, never removed, in this version.

// The artifact categories of a plan item.
const (
	CategorySkill       = "skill"
	CategoryPlugin      = "plugin"
	CategoryMarketplace = "marketplace"
)

// The states of a plan item: declared and present, declared but absent, or
// present but undeclared.
const (
	StateSatisfied = "satisfied"
	StateMissing   = "missing"
	StateExtra     = "extra"
)

// The actions of a plan item. Missing items get "install" (a plugin, or a
// skill through the installer seam), "add" (a marketplace) or "manual"
// (nothing adev can run for it); satisfied and extra items get "none".
const (
	ActionInstall = "install"
	ActionAdd     = "add"
	ActionManual  = "manual"
	ActionNone    = "none"
)

// InstallSkill is the seam through which sync installs a missing skill.
//
// The cli package wires manage's cross-harness copier (task T7) into it at
// init, so the adev binary always runs with the seam filled. It stays a
// function variable because adevfile must not import the mutation layer:
// while the seam is nil (bare package use, or tests that want it quiet),
// Diff classifies missing skills as manual and Apply refuses them.
var InstallSkill func(name, harnessLabel, scope string) (string, error)

// Item is one line of a sync plan: an artifact of one harness scope, its
// state against the manifest, and what applying the plan would do about it.
type Item struct {
	Harness  string `json:"harness"`
	Scope    string `json:"scope"`
	Category string `json:"category"`
	// Name is the artifact's manifest identity: a skill or marketplace
	// name, a plugin key ("name@marketplace").
	Name string `json:"name"`
	// Source is the add source of a marketplace item, when the manifest
	// declares one.
	Source string `json:"source,omitempty"`
	State  string `json:"state"`
	Action string `json:"action"`
}

// Label phrases the item for human output: "claude/user plugin foo@mkt".
func (i Item) Label() string {
	return fmt.Sprintf("%s/%s %s %s", i.Harness, i.Scope, i.Category, i.Name)
}

// Plan is the outcome of diffing a manifest against reality, in a stable
// order: harness detection order, then scope, then category, then the
// manifest's own item order (with extras sorted after the declared items of
// their category).
type Plan struct {
	Items []Item `json:"items"`
}

// Missing counts the declared-but-absent items.
func (p Plan) Missing() int { return p.count(StateMissing) }

// Extra counts the present-but-undeclared items.
func (p Plan) Extra() int { return p.count(StateExtra) }

// InSync reports whether reality satisfies the manifest: nothing declared is
// missing. Extras do not break sync; they are report-only.
func (p Plan) InSync() bool { return p.Missing() == 0 }

func (p Plan) count(state string) int {
	n := 0
	for _, item := range p.Items {
		if item.State == state {
			n++
		}
	}
	return n
}

// realityEntry is the flattened content of one config dir, keyed the same
// way the manifest declares it.
type realityEntry struct {
	skills       map[string]bool
	plugins      map[string]bool
	marketplaces map[string]bool
}

// Diff compares the manifest against the discovered reality (dirs, from a
// scan rooted at root) and returns the plan. Only the scopes the manifest
// declares are diffed: a declared scope owns its config dir (missing items
// and extras are both reported), an undeclared one is left alone.
func Diff(f File, dirs []discovery.ConfigDir, root string) (Plan, error) {
	reality, err := realityByScope(dirs, root)
	if err != nil {
		return Plan{}, err
	}

	var plan Plan
	for _, id := range scaffolding.HarnessesInOrder() {
		label := scaffolding.AIHarnesses[id]
		declared, ok := f.Harnesses[label]
		if !ok {
			continue
		}
		for _, scope := range Scopes {
			entry := declared.Entry(scope)
			if entry == nil {
				continue
			}
			plan.Items = append(plan.Items, diffEntry(label, scope, *entry, reality[label][scope])...)
		}
	}
	return plan, nil
}

// realityByScope flattens the discovered config dirs into per-harness,
// per-scope entries, with the same scope classification export uses. Config
// dirs outside both scopes are ignored: they belong to other projects.
func realityByScope(dirs []discovery.ConfigDir, root string) (map[string]map[string]realityEntry, error) {
	reality := map[string]map[string]realityEntry{}
	for _, dir := range dirs {
		scope, err := classifyScope(dir, root)
		if err != nil {
			return nil, err
		}
		if scope == "" {
			continue
		}

		entry := realityEntry{
			skills:       map[string]bool{},
			plugins:      map[string]bool{},
			marketplaces: map[string]bool{},
		}
		for _, skill := range dir.Skills {
			entry.skills[skill.Name] = true
		}
		for _, plugin := range dir.Plugins {
			entry.plugins[pluginKey(plugin)] = true
		}
		for _, mkt := range dir.Marketplaces {
			entry.marketplaces[mkt.Name] = true
		}

		label := scaffolding.AIHarnesses[dir.Harness]
		if reality[label] == nil {
			reality[label] = map[string]realityEntry{}
		}
		reality[label][scope] = entry
	}
	return reality, nil
}

// diffEntry diffs one declared scope against what its config dir holds:
// first every declared item (satisfied or missing, category by category),
// then the extras.
func diffEntry(label, scope string, declared Entry, actual realityEntry) []Item {
	operable := harnessOperable(label)
	var items []Item

	add := func(category, name, source, action string, present bool) {
		state := StateMissing
		if present {
			state, action = StateSatisfied, ActionNone
		}
		items = append(items, Item{
			Harness:  label,
			Scope:    scope,
			Category: category,
			Name:     name,
			Source:   source,
			State:    state,
			Action:   action,
		})
	}

	for _, name := range declared.Skills {
		action := ActionManual
		if InstallSkill != nil {
			action = ActionInstall
		}
		add(CategorySkill, name, "", action, actual.skills[name])
	}
	for _, key := range declared.Plugins {
		// A bare-name plugin (no marketplace) has nothing to install from,
		// so it stays manual even on an operable harness.
		action := ActionManual
		if operable && strings.Contains(key, "@") {
			action = ActionInstall
		}
		add(CategoryPlugin, key, "", action, actual.plugins[key])
	}
	for _, mkt := range declared.Marketplaces {
		action := ActionManual
		if operable && mkt.Source != "" {
			action = ActionAdd
		}
		add(CategoryMarketplace, mkt.Name, mkt.Source, action, actual.marketplaces[mkt.Name])
	}

	items = append(items, extras(label, scope, CategorySkill, actual.skills, declared.Skills)...)
	items = append(items, extras(label, scope, CategoryPlugin, actual.plugins, declared.Plugins)...)

	declaredMkts := make([]string, 0, len(declared.Marketplaces))
	for _, mkt := range declared.Marketplaces {
		declaredMkts = append(declaredMkts, mkt.Name)
	}
	items = append(items, extras(label, scope, CategoryMarketplace, actual.marketplaces, declaredMkts)...)

	return items
}

// extras lists the reality items of one category the manifest does not
// declare, sorted by name.
func extras(label, scope, category string, actual map[string]bool, declared []string) []Item {
	declaredSet := make(map[string]bool, len(declared))
	for _, name := range declared {
		declaredSet[name] = true
	}

	var names []string
	for name := range actual {
		if !declaredSet[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	items := make([]Item, 0, len(names))
	for _, name := range names {
		items = append(items, Item{
			Harness:  label,
			Scope:    scope,
			Category: category,
			Name:     name,
			State:    StateExtra,
			Action:   ActionNone,
		})
	}
	return items
}

// harnessOperable reports whether label's harness has an operation executor
// (a CLI owning its registry) sync can delegate installs to.
func harnessOperable(label string) bool {
	ops, _ := operationsFor(label)
	return ops != nil
}

// operationsFor resolves the operation executor of one harness label.
func operationsFor(label string) (harness.Operations, error) {
	id, err := scaffolding.ParseHarness(label)
	if err != nil {
		return nil, err
	}
	ad, ok := harness.ForID(id)
	if !ok {
		return nil, fmt.Errorf("no adapter for harness %q", label)
	}
	return ad.Operations(), nil
}

// Apply executes one missing item's action through the harness's own
// executor and returns the executor's output. Items whose action is not
// executable (manual, none, or a harness without an executor) are refused,
// so the caller decides how to report them instead of half-running a plan.
func Apply(item Item) (string, error) {
	if item.State != StateMissing {
		return "", fmt.Errorf("%s is %s, nothing to apply", item.Label(), item.State)
	}

	switch {
	case item.Category == CategorySkill && item.Action == ActionInstall:
		// This call lands on the seam above; see InstallSkill.
		return InstallSkill(item.Name, item.Harness, item.Scope)
	case item.Category == CategoryPlugin && item.Action == ActionInstall:
		ops, err := operationsFor(item.Harness)
		if err != nil || ops == nil {
			return "", fmt.Errorf("%s: the %s harness has no CLI to install plugins", item.Label(), item.Harness)
		}
		return ops.InstallPlugin(item.Name)
	case item.Category == CategoryMarketplace && item.Action == ActionAdd:
		ops, err := operationsFor(item.Harness)
		if err != nil || ops == nil {
			return "", fmt.Errorf("%s: the %s harness has no CLI to add marketplaces", item.Label(), item.Harness)
		}
		return ops.AddMarketplace(item.Source)
	default:
		return "", fmt.Errorf("%s has no executable action (%s)", item.Label(), item.Action)
	}
}
