package harness

import (
	"agentic-developer/internal/scaffolding"
	"path/filepath"
	"testing"
)

// opencode returns the opencode adapter for the tests.
func opencode(t *testing.T) Adapter {
	t.Helper()
	ad, ok := ForID(scaffolding.Opencode)
	if !ok {
		t.Fatal("no opencode adapter registered")
	}
	return ad
}

// TestOpencodeCapabilities verifies the adapter is honest: skills and TS
// plugins, but no marketplaces, no registry, and no operations.
func TestOpencodeCapabilities(t *testing.T) {
	ad := opencode(t)

	kinds := make(map[Kind]string)
	for _, c := range ad.Containers() {
		kinds[c.Kind] = c.Dir
	}
	if kinds[KindSkill] != "skills" || kinds[KindPlugin] != "plugins" {
		t.Fatalf("containers = %v", kinds)
	}
	if _, ok := kinds[KindMarketplace]; ok {
		t.Fatal("opencode reports a marketplace container it does not have")
	}

	if reg := ad.Registry(t.TempDir()); reg.HasInstalled || reg.HasMarketplaces {
		t.Fatalf("opencode reports a registry: %+v", reg)
	}
	if ad.Operations() != nil {
		t.Fatal("opencode reports operations it cannot execute")
	}
}

// TestOpencodePluginMeta verifies TS plugin metadata comes from package.json.
func TestOpencodePluginMeta(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"name": "notify", "version": "0.1.0", "description": "sends a bell"}`)
	meta := opencode(t).PluginMeta(dir)
	if meta.Name != "notify" || meta.Version != "0.1.0" || meta.Description != "sends a bell" {
		t.Fatalf("meta = %+v", meta)
	}

	if got := opencode(t).PluginMeta(t.TempDir()); got != (PluginMeta{}) {
		t.Fatalf("manifest-less meta = %+v, want zero", got)
	}
}

// TestOpencodeValidatePlugin verifies a TS plugin needs its index.ts entry
// point and a parsable package.json.
func TestOpencodeValidatePlugin(t *testing.T) {
	// Healthy: index.ts present, package.json valid.
	healthy := t.TempDir()
	write(t, healthy, "index.ts", "export const plugin = {}\n")
	write(t, healthy, "package.json", `{"name": "ok"}`)
	if issues := opencode(t).ValidatePlugin("ok", healthy); issues != nil {
		t.Fatalf("healthy plugin produced issues: %+v", issues)
	}

	// No entry point: error.
	empty := t.TempDir()
	issues := opencode(t).ValidatePlugin("broken", empty)
	if len(issues) != 1 || issues[0].Check != "plugin-entry-missing" || issues[0].Severity != IssueError {
		t.Fatalf("issues = %+v, want plugin-entry-missing", issues)
	}

	// Broken package.json: flagged alongside the entry point it also lacks.
	bad := t.TempDir()
	write(t, bad, "package.json", "{nope")
	checks := make(map[string]bool)
	for _, issue := range opencode(t).ValidatePlugin("bad", bad) {
		checks[issue.Check] = true
	}
	if !checks["plugin-package-invalid"] || !checks["plugin-entry-missing"] {
		t.Fatalf("checks = %v, want both plugin-package-invalid and plugin-entry-missing", checks)
	}

	// A folder that is not on disk is not this check's business.
	if issues := opencode(t).ValidatePlugin("gone", filepath.Join(t.TempDir(), "nope")); issues != nil {
		t.Fatalf("missing folder produced issues: %+v", issues)
	}
}

// TestCodexCapabilities verifies the Codex adapter is honest: skills and
// prompts only, AGENTS.md awareness, and nothing it does not have.
func TestCodexCapabilities(t *testing.T) {
	ad, ok := ForID(scaffolding.Codex)
	if !ok {
		t.Fatal("no codex adapter registered")
	}

	kinds := make(map[Kind]string)
	for _, c := range ad.Containers() {
		kinds[c.Kind] = c.Dir
	}
	if kinds[KindSkill] != "skills" || kinds[KindPrompt] != "prompts" {
		t.Fatalf("containers = %v", kinds)
	}
	if _, ok := kinds[KindPlugin]; ok {
		t.Fatal("codex reports a plugin container it does not have")
	}
	if _, ok := kinds[KindMarketplace]; ok {
		t.Fatal("codex reports a marketplace container it does not have")
	}

	if reg := ad.Registry(t.TempDir()); reg.HasInstalled || reg.HasMarketplaces {
		t.Fatalf("codex reports a registry: %+v", reg)
	}
	if ad.Operations() != nil {
		t.Fatal("codex reports operations it cannot execute")
	}
	if issues := ad.ValidatePlugin("x", t.TempDir()); issues != nil {
		t.Fatalf("codex validated a plugin it cannot have: %+v", issues)
	}
}
