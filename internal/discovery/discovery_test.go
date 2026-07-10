package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// mkdirs creates every path under base, failing the test on error.
func mkdirs(t *testing.T, base string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Join(base, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// TestScanIncludesUserConfigs verifies the user's home config dirs are always
// prepended to the results, whatever the scan root.
func TestScanIncludesUserConfigs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkdirs(t, home, ".claude/skills/home-skill")

	root := t.TempDir()
	mkdirs(t, root, "proj/.codex/skills/proj-skill")

	dirs, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(dirs) != 2 {
		t.Fatalf("got %d dirs, want 2: %+v", len(dirs), dirs)
	}
	if dirs[0].Path != filepath.Join(home, ".claude") {
		t.Fatalf("first dir = %q, want the user's ~/.claude", dirs[0].Path)
	}
	if len(dirs[0].Skills) != 1 || dirs[0].Skills[0].Name != "home-skill" {
		t.Fatalf("user config skills = %v, want [home-skill]", dirs[0].Skills)
	}
}

// TestScanHomeRootDoesNotDuplicate verifies scanning the home itself lists
// its config dirs once, not twice.
func TestScanHomeRootDoesNotDuplicate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkdirs(t, home, ".claude/skills/home-skill", "proj/.opencode/skills/x")

	dirs, err := Scan(home)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, dir := range dirs {
		if dir.Path == filepath.Join(home, ".claude") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("~/.claude appears %d times, want exactly once", count)
	}
	if len(dirs) != 2 {
		t.Fatalf("got %d dirs, want 2: %+v", len(dirs), dirs)
	}
}

// TestClaudePluginRegistry verifies a Claude config dir with a registry
// reports its installed plugins (with enabled state from settings.json) and
// marketplaces instead of raw cache folders.
func TestClaudePluginRegistry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkdirs(t, home,
		".claude/plugins/cache/some-mkt/tool/1.0.0", // cache noise, must not be listed
		".claude/skills/my-skill",
	)

	installed := `{"version": 2, "plugins": {
		"tool@some-mkt": [{"version": "1.0.0"}],
		"other@some-mkt": [{"version": "0.2.0"}]
	}}`
	known := `{"some-mkt": {"source": {"source": "github", "repo": "acme/some-mkt"}}}`
	settings := `{"enabledPlugins": {"tool@some-mkt": true}}`

	claude := filepath.Join(home, ".claude")
	if err := os.WriteFile(filepath.Join(claude, "plugins", "installed_plugins.json"), []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "plugins", "known_marketplaces.json"), []byte(known), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs, err := Scan(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 {
		t.Fatalf("got %d dirs, want 1: %+v", len(dirs), dirs)
	}

	dir := dirs[0]
	if len(dir.Plugins) != 2 {
		t.Fatalf("plugins = %+v, want 2 registry entries", dir.Plugins)
	}
	if dir.Plugins[0].Name != "other" || dir.Plugins[0].Enabled {
		t.Fatalf("first plugin = %+v, want disabled 'other'", dir.Plugins[0])
	}
	if dir.Plugins[1].Name != "tool" || !dir.Plugins[1].Enabled || dir.Plugins[1].Version != "1.0.0" {
		t.Fatalf("second plugin = %+v, want enabled 'tool' 1.0.0", dir.Plugins[1])
	}
	if len(dir.Marketplaces) != 1 || dir.Marketplaces[0].Source != "github acme/some-mkt" {
		t.Fatalf("marketplaces = %+v, want some-mkt from github acme/some-mkt", dir.Marketplaces)
	}
}

// TestScanHonorsIgnores verifies both ignore layers prune the walk: the
// embedded default list and a .gitignore found in the tree.
func TestScanHonorsIgnores(t *testing.T) {
	home := t.TempDir() // empty home so no user configs interfere
	t.Setenv("HOME", home)

	root := t.TempDir()
	mkdirs(t, root,
		"proj/.claude/skills/real-skill",
		"proj/node_modules/dep/.claude/skills/junk",
		"proj/secret/.codex",
	)
	if err := os.WriteFile(filepath.Join(root, "proj", ".gitignore"), []byte("secret/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(dirs) != 1 {
		t.Fatalf("got %d dirs, want 1: %+v", len(dirs), dirs)
	}
	if dirs[0].Path != filepath.Join(root, "proj", ".claude") {
		t.Fatalf("found %q, want proj/.claude only", dirs[0].Path)
	}
}

// TestGroupSkills verifies cross-path grouping counts every place a skill
// exists and picks up its description.
func TestGroupSkills(t *testing.T) {
	dirs := []ConfigDir{
		{Path: "/a/.claude", Skills: []Skill{{Name: "x", Description: "does x"}, {Name: "y"}}},
		{Path: "/b/.codex", Skills: []Skill{{Name: "x"}}},
	}

	groups := GroupSkills(dirs)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(groups), groups)
	}
	if groups[0].Name != "x" || len(groups[0].Locations) != 2 {
		t.Fatalf("group x = %+v, want 2 locations", groups[0])
	}
	if groups[0].Description != "does x" {
		t.Fatalf("group x description = %q", groups[0].Description)
	}
	if groups[1].Name != "y" || len(groups[1].Locations) != 1 {
		t.Fatalf("group y = %+v, want 1 location", groups[1])
	}
}

// TestGroupPluginsByIdentity verifies the same plugin name in two
// marketplaces stays two groups, while the same identity in two config dirs
// merges.
func TestGroupPluginsByIdentity(t *testing.T) {
	dirs := []ConfigDir{
		{Path: "/a/.claude", Plugins: []Plugin{
			{Name: "ai", Marketplace: "mkt1", Version: "1.0"},
			{Name: "ai", Marketplace: "mkt2"},
		}},
		{Path: "/b/.claude", Plugins: []Plugin{{Name: "ai", Marketplace: "mkt1"}}},
	}

	groups := GroupPlugins(dirs)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(groups), groups)
	}
	if groups[0].Key != "ai@mkt1" || len(groups[0].Locations) != 2 || groups[0].Version != "1.0" {
		t.Fatalf("group ai@mkt1 = %+v", groups[0])
	}
	if groups[1].Key != "ai@mkt2" || len(groups[1].Locations) != 1 {
		t.Fatalf("group ai@mkt2 = %+v", groups[1])
	}
}

// TestReadClaudeRegistry verifies the raw registry reader parses the three
// registry files and reports which ones were readable.
func TestReadClaudeRegistry(t *testing.T) {
	configDir := t.TempDir()
	mkdirs(t, configDir, "plugins")

	installed := `{"version": 2, "plugins": {
		"tool@mkt": [{"version": "1.0.0", "installPath": "/cache/mkt/tool/1.0.0"}]
	}}`
	known := `{"mkt": {
		"source": {"source": "directory", "path": "/src/mkt"},
		"installLocation": "/src/mkt"
	}}`
	settings := `{"enabledPlugins": {"tool@mkt": true, "ghost@mkt": true}}`

	if err := os.WriteFile(filepath.Join(configDir, "plugins", "installed_plugins.json"), []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "plugins", "known_marketplaces.json"), []byte(known), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := ReadClaudeRegistry(configDir)
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
	empty := ReadClaudeRegistry(t.TempDir())
	if empty.HasInstalled || empty.HasMarketplaces || empty.EnabledPlugins != nil {
		t.Fatalf("empty dir registry = %+v, want nothing readable", empty)
	}
}

// TestParseFrontmatter verifies the SKILL.md frontmatter reader.
func TestParseFrontmatter(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "SKILL.md")
	content := "---\nname: my-skill\ndescription: \"Does things. Use when: things need doing.\"\n---\n\n# Body\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	meta := ParseFrontmatter(file)
	if meta["name"] != "my-skill" {
		t.Fatalf("name = %q", meta["name"])
	}
	if meta["description"] != "Does things. Use when: things need doing." {
		t.Fatalf("description = %q", meta["description"])
	}

	// A file with no frontmatter yields nothing.
	if err := os.WriteFile(file, []byte("# Just markdown\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ParseFrontmatter(file) != nil {
		t.Fatal("expected nil for a file without frontmatter")
	}
}
