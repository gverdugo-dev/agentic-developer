package doctor

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/scaffolding"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write creates file under dir (creating parents) with content.
func write(t *testing.T, dir, file, content string) string {
	t.Helper()
	path := filepath.Join(dir, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// skillFixture creates a skill folder holding a SKILL.md with content, and
// returns the ConfigDir that discovery would report for it. A "" content
// leaves the folder without a SKILL.md.
func skillFixture(t *testing.T, content string) discovery.ConfigDir {
	t.Helper()
	configDir := t.TempDir()
	skillPath := filepath.Join(configDir, "skills", "my-skill")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		write(t, skillPath, "SKILL.md", content)
	}
	return discovery.ConfigDir{
		Path:    configDir,
		Harness: scaffolding.Claude,
		Skills:  []discovery.Skill{{Name: "my-skill", Path: skillPath}},
	}
}

// only asserts findings holds exactly one finding per expected check id, in
// any order, and returns them indexed by check.
func only(t *testing.T, findings []Finding, checks ...string) map[string]Finding {
	t.Helper()
	if len(findings) != len(checks) {
		t.Fatalf("got %d findings %+v, want checks %v", len(findings), findings, checks)
	}
	byCheck := make(map[string]Finding, len(findings))
	for _, f := range findings {
		byCheck[f.Check] = f
	}
	for _, check := range checks {
		if _, ok := byCheck[check]; !ok {
			t.Fatalf("missing finding %q in %+v", check, findings)
		}
	}
	return byCheck
}

const healthySkill = "---\nname: my-skill\ndescription: does things\n---\n\n# Body\n"

// TestHealthySkill verifies a well-formed skill yields no findings.
func TestHealthySkill(t *testing.T) {
	if findings := Check([]discovery.ConfigDir{skillFixture(t, healthySkill)}); findings != nil {
		t.Fatalf("healthy skill produced findings: %+v", findings)
	}
}

// TestSkillMdMissing verifies a skill folder without SKILL.md is an error.
func TestSkillMdMissing(t *testing.T) {
	dir := skillFixture(t, "")
	byCheck := only(t, Check([]discovery.ConfigDir{dir}), "skill-md-missing")

	f := byCheck["skill-md-missing"]
	if f.Severity != Error {
		t.Fatalf("severity = %v, want Error", f.Severity)
	}
	if f.Path != dir.Skills[0].Path {
		t.Fatalf("path = %q, want the skill folder", f.Path)
	}
	if f.FixHint == "" {
		t.Fatal("finding has no fix hint")
	}
}

// TestSkillFrontmatterMissing verifies a SKILL.md without a frontmatter
// block is an error.
func TestSkillFrontmatterMissing(t *testing.T) {
	dir := skillFixture(t, "# Just markdown, no frontmatter\n")
	byCheck := only(t, Check([]discovery.ConfigDir{dir}), "skill-frontmatter-missing")
	if byCheck["skill-frontmatter-missing"].Severity != Error {
		t.Fatal("frontmatter-missing should be an error")
	}
}

// TestSkillFrontmatterFields verifies a frontmatter without description is
// an error and without name a warning.
func TestSkillFrontmatterFields(t *testing.T) {
	dir := skillFixture(t, "---\nother: value\n---\n\n# Body\n")
	byCheck := only(t, Check([]discovery.ConfigDir{dir}),
		"skill-description-missing", "skill-name-missing")

	if byCheck["skill-description-missing"].Severity != Error {
		t.Fatal("description-missing should be an error: the skill never triggers")
	}
	if byCheck["skill-name-missing"].Severity != Warning {
		t.Fatal("name-missing should be a warning")
	}
}

// TestSkillMdTooLong verifies the 200-line house-style cap.
func TestSkillMdTooLong(t *testing.T) {
	content := healthySkill + strings.Repeat("filler line\n", maxSkillLines)
	dir := skillFixture(t, content)
	byCheck := only(t, Check([]discovery.ConfigDir{dir}), "skill-md-too-long")

	f := byCheck["skill-md-too-long"]
	if f.Severity != Warning {
		t.Fatalf("severity = %v, want Warning", f.Severity)
	}
	if !strings.Contains(f.Message, "206 lines") {
		t.Fatalf("message misses the line count: %q", f.Message)
	}

	// Exactly the cap is fine.
	exact := healthySkill + strings.Repeat("x\n", maxSkillLines-6)
	if findings := Check([]discovery.ConfigDir{skillFixture(t, exact)}); findings != nil {
		t.Fatalf("a %d-line SKILL.md produced findings: %+v", maxSkillLines, findings)
	}
}

// TestPluginManifest verifies the plugin.json checks: missing manifest,
// invalid JSON, missing name, and a healthy one.
func TestPluginManifest(t *testing.T) {
	cases := []struct {
		name     string
		manifest string // "" = no file
		want     string // "" = no finding
		severity Severity
	}{
		{"missing", "", "plugin-manifest-missing", Error},
		{"invalid", "{not json", "plugin-manifest-invalid", Error},
		{"unnamed", `{"description": "x"}`, "plugin-name-missing", Warning},
		{"healthy", `{"name": "my-plugin"}`, "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pluginPath := t.TempDir()
			if tc.manifest != "" {
				write(t, pluginPath, ".claude-plugin/plugin.json", tc.manifest)
			}
			dir := discovery.ConfigDir{
				Path:    t.TempDir(),
				Harness: scaffolding.Claude,
				Plugins: []discovery.Plugin{{Name: "my-plugin", Path: pluginPath}},
			}

			findings := Check([]discovery.ConfigDir{dir})
			if tc.want == "" {
				if findings != nil {
					t.Fatalf("healthy plugin produced findings: %+v", findings)
				}
				return
			}
			byCheck := only(t, findings, tc.want)
			if byCheck[tc.want].Severity != tc.severity {
				t.Fatalf("severity = %v, want %v", byCheck[tc.want].Severity, tc.severity)
			}
		})
	}
}

// TestMarketplaceManifest verifies the marketplace.json checks mirror the
// plugin ones.
func TestMarketplaceManifest(t *testing.T) {
	mktPath := t.TempDir()
	write(t, mktPath, ".claude-plugin/marketplace.json", "{broken")
	dir := discovery.ConfigDir{
		Path:         t.TempDir(),
		Harness:      scaffolding.Claude,
		Marketplaces: []discovery.Marketplace{{Name: "my-mkt", Path: mktPath}},
	}
	only(t, Check([]discovery.ConfigDir{dir}), "marketplace-manifest-invalid")

	// A marketplace whose folder is not on disk is the registry checks'
	// business, not a manifest finding.
	gone := discovery.ConfigDir{
		Path:         t.TempDir(),
		Harness:      scaffolding.Claude,
		Marketplaces: []discovery.Marketplace{{Name: "gone", Path: filepath.Join(t.TempDir(), "nope")}},
	}
	if findings := Check([]discovery.ConfigDir{gone}); findings != nil {
		t.Fatalf("missing folder produced manifest findings: %+v", findings)
	}
}

// TestValidationComesFromTheAdapter verifies each harness's own validators
// run: plugin.json checks never fire for opencode (whose plugins are not
// Claude-shaped), while its adapter flags a TS plugin without its index.ts
// entry point.
func TestValidationComesFromTheAdapter(t *testing.T) {
	pluginPath := t.TempDir()
	write(t, pluginPath, "package.json", `{"name": "ts-plugin"}`)
	dir := discovery.ConfigDir{
		Path:    t.TempDir(),
		Harness: scaffolding.Opencode,
		Plugins: []discovery.Plugin{{Name: "ts-plugin", Path: pluginPath}},
	}
	byCheck := only(t, Check([]discovery.ConfigDir{dir}), "plugin-entry-missing")
	if byCheck["plugin-entry-missing"].Severity != Error {
		t.Fatal("a TS plugin without index.ts should be an error")
	}

	// A healthy TS plugin yields nothing (and no Claude-shaped findings).
	write(t, pluginPath, "index.ts", "export const plugin = {}\n")
	if findings := Check([]discovery.ConfigDir{dir}); findings != nil {
		t.Fatalf("healthy opencode plugin produced findings: %+v", findings)
	}
}

// registryFixture builds a Claude config dir with the given registry files.
func registryFixture(t *testing.T, installed, known, settings string) discovery.ConfigDir {
	t.Helper()
	configDir := t.TempDir()
	if installed != "" {
		write(t, configDir, "plugins/installed_plugins.json", installed)
	}
	if known != "" {
		write(t, configDir, "plugins/known_marketplaces.json", known)
	}
	if settings != "" {
		write(t, configDir, "settings.json", settings)
	}
	return discovery.ConfigDir{Path: configDir, Harness: scaffolding.Claude}
}

// TestEnabledNotInstalled verifies a plugin enabled in settings.json without
// a registry entry is an error, and a disabled one is not.
func TestEnabledNotInstalled(t *testing.T) {
	dir := registryFixture(t,
		`{"version": 2, "plugins": {}}`,
		"",
		`{"enabledPlugins": {"ghost@mkt": true, "off@mkt": false}}`,
	)

	byCheck := only(t, Check([]discovery.ConfigDir{dir}), "plugin-enabled-not-installed")
	f := byCheck["plugin-enabled-not-installed"]
	if f.Severity != Error || !strings.Contains(f.Message, "ghost@mkt") {
		t.Fatalf("finding = %+v, want error about ghost@mkt", f)
	}
}

// TestPluginCacheMissing verifies a registry install whose install path is
// gone is an error.
func TestPluginCacheMissing(t *testing.T) {
	configDir := t.TempDir()
	present := filepath.Join(configDir, "plugins", "cache", "mkt", "here", "1.0.0")
	if err := os.MkdirAll(present, 0o755); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(configDir, "plugins", "cache", "mkt", "gone", "1.0.0")

	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {
		"here@mkt": [{"version": "1.0.0", "installPath": `+jsonString(present)+`}],
		"gone@mkt": [{"version": "1.0.0", "installPath": `+jsonString(gone)+`}]
	}}`)
	// The present install needs a manifest so the manifest check stays quiet.
	write(t, present, ".claude-plugin/plugin.json", `{"name": "here"}`)

	dir := discovery.ConfigDir{Path: configDir, Harness: scaffolding.Claude}
	byCheck := only(t, Check([]discovery.ConfigDir{dir}), "plugin-cache-missing")
	f := byCheck["plugin-cache-missing"]
	if f.Severity != Error || f.Path != gone || !strings.Contains(f.Message, "gone@mkt") {
		t.Fatalf("finding = %+v, want error at %q about gone@mkt", f, gone)
	}
}

// TestCacheOrphan verifies a cache folder no registry entry claims is a
// warning, and a claimed one is not.
func TestCacheOrphan(t *testing.T) {
	configDir := t.TempDir()
	claimed := filepath.Join(configDir, "plugins", "cache", "mkt", "claimed", "1.0.0")
	orphan := filepath.Join(configDir, "plugins", "cache", "mkt", "orphan", "1.0.0")
	for _, p := range []string{claimed, orphan} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, claimed, ".claude-plugin/plugin.json", `{"name": "claimed"}`)

	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {
		"claimed@mkt": [{"version": "1.0.0", "installPath": `+jsonString(claimed)+`}]
	}}`)

	dir := discovery.ConfigDir{Path: configDir, Harness: scaffolding.Claude}
	byCheck := only(t, Check([]discovery.ConfigDir{dir}), "cache-orphan")
	f := byCheck["cache-orphan"]
	if f.Severity != Warning || !strings.Contains(f.Message, "orphan@mkt") {
		t.Fatalf("finding = %+v, want warning about orphan@mkt", f)
	}
	if f.Path != filepath.Join(configDir, "plugins", "cache", "mkt", "orphan") {
		t.Fatalf("path = %q, want the orphaned cache folder", f.Path)
	}
}

// TestCacheWithoutRegistryIsQuiet verifies cache folders are not flagged
// when there is no readable registry to compare against.
func TestCacheWithoutRegistryIsQuiet(t *testing.T) {
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "plugins", "cache", "mkt", "p", "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := discovery.ConfigDir{Path: configDir, Harness: scaffolding.Claude}
	if findings := Check([]discovery.ConfigDir{dir}); findings != nil {
		t.Fatalf("cache without registry produced findings: %+v", findings)
	}
}

// TestMarketplaceDirMissing verifies a directory-source marketplace whose
// path was deleted is an error, while github sources are never path-checked.
func TestMarketplaceDirMissing(t *testing.T) {
	gonePath := filepath.Join(t.TempDir(), "deleted-marketplace")
	dir := registryFixture(t, "", `{
		"dead": {"source": {"source": "directory", "path": `+jsonString(gonePath)+`}, "installLocation": `+jsonString(gonePath)+`},
		"remote": {"source": {"source": "github", "repo": "acme/mkt"}, "installLocation": "/nowhere"}
	}`, "")

	byCheck := only(t, Check([]discovery.ConfigDir{dir}), "marketplace-dir-missing")
	f := byCheck["marketplace-dir-missing"]
	if f.Severity != Error || f.Path != gonePath || !strings.Contains(f.Message, `"dead"`) {
		t.Fatalf("finding = %+v, want error at %q about dead", f, gonePath)
	}
}

// TestFindingsSorted verifies errors come before warnings, then by path.
func TestFindingsSorted(t *testing.T) {
	// One skill with a warning (too long) in a path sorting before another
	// skill with an error (no SKILL.md).
	configDir := t.TempDir()
	aPath := filepath.Join(configDir, "skills", "a-long")
	bPath := filepath.Join(configDir, "skills", "b-broken")
	for _, p := range []string{aPath, bPath} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, aPath, "SKILL.md", healthySkill+strings.Repeat("x\n", maxSkillLines))

	dir := discovery.ConfigDir{
		Path:    configDir,
		Harness: scaffolding.Claude,
		Skills: []discovery.Skill{
			{Name: "a-long", Path: aPath},
			{Name: "b-broken", Path: bPath},
		},
	}

	findings := Check([]discovery.ConfigDir{dir})
	if len(findings) != 2 {
		t.Fatalf("got %d findings: %+v", len(findings), findings)
	}
	if findings[0].Check != "skill-md-missing" || findings[1].Check != "skill-md-too-long" {
		t.Fatalf("order = [%s, %s], want the error first", findings[0].Check, findings[1].Check)
	}
}

// TestSeverityJSON verifies severities encode as their names.
func TestSeverityJSON(t *testing.T) {
	raw, err := Error.MarshalJSON()
	if err != nil || string(raw) != `"error"` {
		t.Fatalf("Error marshals to %s (%v)", raw, err)
	}
	raw, err = Warning.MarshalJSON()
	if err != nil || string(raw) != `"warning"` {
		t.Fatalf("Warning marshals to %s (%v)", raw, err)
	}
}

// jsonString quotes s as a JSON string literal.
func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}
