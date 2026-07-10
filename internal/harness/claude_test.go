package harness

import (
	"agentic-developer/internal/scaffolding"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claude returns the Claude adapter for the tests.
func claude(t *testing.T) Adapter {
	t.Helper()
	ad, ok := ForID(scaffolding.Claude)
	if !ok {
		t.Fatal("no Claude adapter registered")
	}
	return ad
}

// TestClaudeRegistry verifies the raw registry reader parses the three
// registry files and reports which ones were readable.
func TestClaudeRegistry(t *testing.T) {
	configDir := t.TempDir()

	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {
		"tool@mkt": [{"version": "1.0.0", "installPath": "/cache/mkt/tool/1.0.0"}]
	}}`)
	write(t, configDir, "plugins/known_marketplaces.json", `{"mkt": {
		"source": {"source": "directory", "path": "/src/mkt"},
		"installLocation": "/src/mkt"
	}}`)
	write(t, configDir, "settings.json", `{"enabledPlugins": {"tool@mkt": true, "ghost@mkt": true}}`)

	reg := claude(t).Registry(configDir)
	if !reg.HasInstalled || !reg.HasMarketplaces {
		t.Fatalf("Has flags = %v/%v, want true/true", reg.HasInstalled, reg.HasMarketplaces)
	}
	installs := reg.InstalledPlugins["tool@mkt"]
	if len(installs) != 1 || installs[0].Version != "1.0.0" || installs[0].InstallPath != "/cache/mkt/tool/1.0.0" {
		t.Fatalf("installs = %+v", installs)
	}
	mkt := reg.KnownMarketplaces["mkt"]
	if mkt.Kind != "directory" || mkt.Path != "/src/mkt" || mkt.InstallLocation != "/src/mkt" {
		t.Fatalf("marketplace = %+v", mkt)
	}
	if mkt.SourceLabel() != "directory /src/mkt" {
		t.Fatalf("source label = %q", mkt.SourceLabel())
	}
	if !reg.EnabledPlugins["ghost@mkt"] {
		t.Fatal("enabledPlugins missing ghost@mkt")
	}

	// An empty dir has no registry at all.
	empty := claude(t).Registry(t.TempDir())
	if empty.HasInstalled || empty.HasMarketplaces || empty.EnabledPlugins != nil {
		t.Fatalf("empty dir registry = %+v, want nothing readable", empty)
	}
}

// TestClaudeMetas verifies the plugin.json and marketplace.json readers.
func TestClaudeMetas(t *testing.T) {
	pluginDir := t.TempDir()
	write(t, pluginDir, ".claude-plugin/plugin.json",
		`{"name": "my-plugin", "version": "1.2.3", "description": "does things"}`)
	meta := claude(t).PluginMeta(pluginDir)
	if meta.Name != "my-plugin" || meta.Version != "1.2.3" || meta.Description != "does things" {
		t.Fatalf("plugin meta = %+v", meta)
	}
	if got := claude(t).PluginMeta(t.TempDir()); got != (PluginMeta{}) {
		t.Fatalf("manifest-less plugin meta = %+v, want zero", got)
	}

	mktDir := t.TempDir()
	write(t, mktDir, ".claude-plugin/marketplace.json",
		`{"name": "mkt", "plugins": [{"name": "one"}, {"name": ""}, {"name": "two"}]}`)
	mkt := claude(t).MarketplaceMeta(mktDir)
	if mkt.Name != "mkt" || len(mkt.PluginNames) != 2 || mkt.PluginNames[0] != "one" || mkt.PluginNames[1] != "two" {
		t.Fatalf("marketplace meta = %+v", mkt)
	}
}

// TestClaudeValidateManifests verifies the manifest validators: missing
// file, invalid JSON, missing name, healthy, and the folder-not-on-disk
// guard (that case belongs to the registry checks).
func TestClaudeValidateManifests(t *testing.T) {
	cases := []struct {
		name     string
		manifest string // "" = no file
		want     string // "" = no issue
		severity IssueSeverity
	}{
		{"missing", "", "plugin-manifest-missing", IssueError},
		{"invalid", "{not json", "plugin-manifest-invalid", IssueError},
		{"unnamed", `{"description": "x"}`, "plugin-name-missing", IssueWarning},
		{"healthy", `{"name": "my-plugin"}`, "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.manifest != "" {
				write(t, dir, ".claude-plugin/plugin.json", tc.manifest)
			}
			issues := claude(t).ValidatePlugin("my-plugin", dir)
			if tc.want == "" {
				if issues != nil {
					t.Fatalf("healthy plugin produced issues: %+v", issues)
				}
				return
			}
			if len(issues) != 1 || issues[0].Check != tc.want || issues[0].Severity != tc.severity {
				t.Fatalf("issues = %+v, want one %s", issues, tc.want)
			}
		})
	}

	// Marketplace manifests mirror the plugin ones.
	mktDir := t.TempDir()
	write(t, mktDir, ".claude-plugin/marketplace.json", "{broken")
	issues := claude(t).ValidateMarketplace("my-mkt", mktDir)
	if len(issues) != 1 || issues[0].Check != "marketplace-manifest-invalid" {
		t.Fatalf("issues = %+v, want marketplace-manifest-invalid", issues)
	}

	// A folder that is not on disk is the registry checks' business.
	if issues := claude(t).ValidatePlugin("gone", filepath.Join(t.TempDir(), "nope")); issues != nil {
		t.Fatalf("missing folder produced manifest issues: %+v", issues)
	}
}

// TestClaudeValidateRegistry verifies the registry cross-checks: ghost
// enabled plugins, gone install paths, orphaned cache folders, and dead
// directory-source marketplaces.
func TestClaudeValidateRegistry(t *testing.T) {
	configDir := t.TempDir()
	present := filepath.Join(configDir, "plugins", "cache", "mkt", "here", "1.0.0")
	orphan := filepath.Join(configDir, "plugins", "cache", "mkt", "orphan", "1.0.0")
	for _, p := range []string{present, orphan} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gone := filepath.Join(configDir, "plugins", "cache", "mkt", "gone", "1.0.0")
	deadPath := filepath.Join(t.TempDir(), "deleted-marketplace")

	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {
		"here@mkt": [{"version": "1.0.0", "installPath": `+jsonString(present)+`}],
		"gone@mkt": [{"version": "1.0.0", "installPath": `+jsonString(gone)+`}]
	}}`)
	write(t, configDir, "plugins/known_marketplaces.json", `{
		"dead": {"source": {"source": "directory", "path": `+jsonString(deadPath)+`}, "installLocation": `+jsonString(deadPath)+`},
		"remote": {"source": {"source": "github", "repo": "acme/mkt"}, "installLocation": "/nowhere"}
	}`)
	write(t, configDir, "settings.json", `{"enabledPlugins": {"ghost@mkt": true, "here@mkt": true, "off@mkt": false}}`)

	issues := claude(t).ValidateRegistry(configDir)
	byCheck := make(map[string]Issue, len(issues))
	for _, issue := range issues {
		byCheck[issue.Check] = issue
	}
	if len(issues) != 4 {
		t.Fatalf("got %d issues %+v, want 4", len(issues), issues)
	}

	if f := byCheck["plugin-enabled-not-installed"]; f.Severity != IssueError || !strings.Contains(f.Message, "ghost@mkt") {
		t.Fatalf("ghost issue = %+v", f)
	}
	if f := byCheck["plugin-cache-missing"]; f.Severity != IssueError || f.Path != gone {
		t.Fatalf("cache-missing issue = %+v", f)
	}
	if f := byCheck["cache-orphan"]; f.Severity != IssueWarning || f.Path != filepath.Join(configDir, "plugins", "cache", "mkt", "orphan") {
		t.Fatalf("orphan issue = %+v", f)
	}
	if f := byCheck["marketplace-dir-missing"]; f.Severity != IssueError || f.Path != deadPath {
		t.Fatalf("dead marketplace issue = %+v", f)
	}

	// No registry at all: nothing to cross-reference, even with cache noise.
	quiet := t.TempDir()
	if err := os.MkdirAll(filepath.Join(quiet, "plugins", "cache", "mkt", "p", "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if issues := claude(t).ValidateRegistry(quiet); issues != nil {
		t.Fatalf("registry-less dir produced issues: %+v", issues)
	}
}

// TestClaudeOperationsBuildTheRightCommands verifies the claude CLI
// delegation uses the exact argument shapes, via a stubbed executor.
func TestClaudeOperationsBuildTheRightCommands(t *testing.T) {
	var got [][]string
	orig := ClaudeExec
	ClaudeExec = func(args ...string) (string, error) {
		got = append(got, args)
		return "ok", nil
	}
	defer func() { ClaudeExec = orig }()

	ops := claude(t).Operations()
	if ops == nil {
		t.Fatal("Claude adapter has no operations")
	}

	steps := []func() (string, error){
		func() (string, error) { return ops.SetPluginEnabled("a@m", true) },
		func() (string, error) { return ops.SetPluginEnabled("a@m", false) },
		func() (string, error) { return ops.InstallPlugin("a@m") },
		func() (string, error) { return ops.UninstallPlugin("a@m") },
		func() (string, error) { return ops.AddMarketplace("owner/repo") },
		func() (string, error) { return ops.RemoveMarketplace("mkt") },
	}
	for i, step := range steps {
		if _, err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	want := [][]string{
		{"plugin", "enable", "a@m"},
		{"plugin", "disable", "a@m"},
		{"plugin", "install", "a@m"},
		{"plugin", "uninstall", "a@m"},
		{"plugin", "marketplace", "add", "owner/repo"},
		{"plugin", "marketplace", "remove", "mkt"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d calls, want %d", len(got), len(want))
	}
	for i := range want {
		if strings.Join(got[i], " ") != strings.Join(want[i], " ") {
			t.Fatalf("call %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// jsonString quotes s as a JSON string literal.
func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}
