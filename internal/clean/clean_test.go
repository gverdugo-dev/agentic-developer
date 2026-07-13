package clean

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/doctor"
	"agentic-developer/internal/harness"
	"agentic-developer/internal/scaffolding"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configDirFixture creates a .claude config dir under a temp root. Paths
// carry the .claude component so ActionRemoveDir candidates stay inside what
// manage.DeleteArtifact accepts.
func configDirFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

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

// jsonString quotes s as a JSON string literal.
func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}

// claudeDir wraps a config dir path as the ConfigDir discovery would report.
func claudeDir(path string) discovery.ConfigDir {
	return discovery.ConfigDir{Path: path, Harness: scaffolding.Claude}
}

// TestCollectStaleVersions verifies a cached version no install record
// claims is a candidate, while the installed version is kept.
func TestCollectStaleVersions(t *testing.T) {
	configDir := configDirFixture(t)
	installed := filepath.Join(configDir, "plugins", "cache", "mkt", "p", "2.0.0")
	stale := filepath.Join(configDir, "plugins", "cache", "mkt", "p", "1.0.0")
	write(t, installed, "plugin.json", `{"name": "p"}`)
	write(t, stale, "plugin.json", `{"name": "p"}`)

	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {
		"p@mkt": [{"version": "2.0.0", "installPath": `+jsonString(installed)+`}]
	}}`)

	candidates := Collect([]discovery.ConfigDir{claudeDir(configDir)}, nil)
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates %+v, want 1", len(candidates), candidates)
	}
	c := candidates[0]
	if c.Kind != KindStaleVersion || c.Path != stale || c.Action != ActionRemoveDir {
		t.Fatalf("candidate = %+v, want stale-version remove-dir at %q", c, stale)
	}
	if c.Size <= 0 {
		t.Fatalf("size = %d, want the stale dir's bytes", c.Size)
	}
	if !strings.Contains(c.Reason, `"1.0.0"`) || !strings.Contains(c.Reason, "p@mkt") {
		t.Fatalf("reason = %q, want the version and the plugin key", c.Reason)
	}
}

// TestCollectStaleSkippedWithoutInstallPath verifies that when no install
// record carries a path there is nothing safe to compare against, so no
// version is flagged.
func TestCollectStaleSkippedWithoutInstallPath(t *testing.T) {
	configDir := configDirFixture(t)
	write(t, filepath.Join(configDir, "plugins", "cache", "mkt", "p", "1.0.0"), "f", "x")

	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {
		"p@mkt": [{"version": "1.0.0", "installPath": ""}]
	}}`)

	if candidates := Collect([]discovery.ConfigDir{claudeDir(configDir)}, nil); len(candidates) != 0 {
		t.Fatalf("got candidates %+v, want none", candidates)
	}
}

// TestCollectCacheOrphan verifies a cache folder with no registry entry is
// an orphan candidate removed from disk.
func TestCollectCacheOrphan(t *testing.T) {
	configDir := configDirFixture(t)
	orphan := filepath.Join(configDir, "plugins", "cache", "mkt", "orphan")
	write(t, filepath.Join(orphan, "1.0.0"), "f", "some bytes")

	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {}}`)

	candidates := Collect([]discovery.ConfigDir{claudeDir(configDir)}, nil)
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates %+v, want 1", len(candidates), candidates)
	}
	c := candidates[0]
	if c.Kind != KindCacheOrphan || c.Path != orphan || c.Action != ActionRemoveDir || c.Size <= 0 {
		t.Fatalf("candidate = %+v, want cache-orphan remove-dir at %q with a size", c, orphan)
	}
}

// TestCollectCacheWithoutRegistryIsQuiet verifies the cache is left alone
// when there is no readable registry to compare against.
func TestCollectCacheWithoutRegistryIsQuiet(t *testing.T) {
	configDir := configDirFixture(t)
	write(t, filepath.Join(configDir, "plugins", "cache", "mkt", "p", "1.0.0"), "f", "x")

	if candidates := Collect([]discovery.ConfigDir{claudeDir(configDir)}, nil); len(candidates) != 0 {
		t.Fatalf("got candidates %+v, want none without a registry", candidates)
	}
}

// TestCollectDeadMarketplace verifies a directory-source marketplace whose
// path is gone becomes a remove-marketplace candidate, while live directory
// and github sources never do.
func TestCollectDeadMarketplace(t *testing.T) {
	configDir := configDirFixture(t)
	gone := filepath.Join(t.TempDir(), "deleted-marketplace")
	alive := t.TempDir()

	write(t, configDir, "plugins/known_marketplaces.json", `{
		"dead": {"source": {"source": "directory", "path": `+jsonString(gone)+`}, "installLocation": `+jsonString(gone)+`},
		"alive": {"source": {"source": "directory", "path": `+jsonString(alive)+`}, "installLocation": `+jsonString(alive)+`},
		"remote": {"source": {"source": "github", "repo": "acme/mkt"}, "installLocation": "/nowhere"}
	}`)

	candidates := Collect([]discovery.ConfigDir{claudeDir(configDir)}, nil)
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates %+v, want 1", len(candidates), candidates)
	}
	c := candidates[0]
	if c.Kind != KindDeadMarketplace || c.Action != ActionRemoveMarketplace || c.Arg != "dead" || c.Path != gone {
		t.Fatalf("candidate = %+v, want remove-marketplace dead at %q", c, gone)
	}
}

// TestCollectBrokenArtifacts verifies the doctor findings whose remedy is
// removal map to candidates: broken skills and folder plugins are deleted
// from disk, registry plugins are uninstalled through the claude CLI, and
// registered marketplaces with manifest trouble are left alone.
func TestCollectBrokenArtifacts(t *testing.T) {
	configDir := configDirFixture(t)
	skillPath := filepath.Join(configDir, "skills", "broken")
	folderPlugin := filepath.Join(configDir, "plugins", "folder-plugin")
	registryPlugin := filepath.Join(configDir, "plugins", "cache", "mkt", "reg", "1.0.0")
	folderMkt := filepath.Join(configDir, "marketplaces", "folder-mkt")
	registryMkt := filepath.Join(configDir, "registered-mkt")
	for _, p := range []string{skillPath, folderPlugin, registryPlugin, folderMkt, registryMkt} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	dirs := []discovery.ConfigDir{{
		Path:    configDir,
		Harness: scaffolding.Claude,
		Plugins: []discovery.Plugin{
			{Name: "folder-plugin", Path: folderPlugin},
			{Name: "reg", Marketplace: "mkt", Path: registryPlugin},
		},
		Marketplaces: []discovery.Marketplace{
			{Name: "folder-mkt", Path: folderMkt},
			{Name: "registered-mkt", Source: "github acme/mkt", Path: registryMkt},
		},
	}}
	findings := []doctor.Finding{
		{Check: "skill-md-missing", Path: skillPath, Message: `skill "broken" has no SKILL.md, so it can never load`},
		{Check: "plugin-manifest-missing", Path: folderPlugin, Message: "folder plugin manifest gone"},
		{Check: "plugin-manifest-missing", Path: registryPlugin, Message: "registry plugin manifest gone"},
		{Check: "marketplace-manifest-missing", Path: folderMkt, Message: "folder marketplace manifest gone"},
		{Check: "marketplace-manifest-missing", Path: registryMkt, Message: "registered marketplace manifest gone"},
		{Check: "skill-md-too-long", Path: skillPath, Message: "fixable, not removable"},
	}

	candidates := Collect(dirs, findings)
	if len(candidates) != 4 {
		t.Fatalf("got %d candidates %+v, want 4", len(candidates), candidates)
	}
	byPath := make(map[string]Candidate, len(candidates))
	for _, c := range candidates {
		if c.Kind != KindBrokenArtifact {
			t.Fatalf("candidate = %+v, want kind %q", c, KindBrokenArtifact)
		}
		byPath[c.Path] = c
	}

	if c := byPath[skillPath]; c.Action != ActionRemoveDir {
		t.Fatalf("broken skill candidate = %+v, want remove-dir", c)
	}
	if c := byPath[folderPlugin]; c.Action != ActionRemoveDir {
		t.Fatalf("folder plugin candidate = %+v, want remove-dir", c)
	}
	if c := byPath[registryPlugin]; c.Action != ActionUninstall || c.Arg != "reg@mkt" {
		t.Fatalf("registry plugin candidate = %+v, want uninstall reg@mkt", c)
	}
	if c := byPath[folderMkt]; c.Action != ActionRemoveDir {
		t.Fatalf("folder marketplace candidate = %+v, want remove-dir", c)
	}
	if _, ok := byPath[registryMkt]; ok {
		t.Fatal("a registered marketplace with a broken manifest must not be a candidate")
	}
}

// TestCollectSkipsOtherHarnesses verifies cache and registry checks never
// fire for non-Claude config dirs.
func TestCollectSkipsOtherHarnesses(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".codex")
	write(t, filepath.Join(configDir, "plugins", "cache", "mkt", "p"), "f", "x")
	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {}}`)

	dirs := []discovery.ConfigDir{{Path: configDir, Harness: scaffolding.Codex}}
	if candidates := Collect(dirs, nil); len(candidates) != 0 {
		t.Fatalf("got candidates %+v for a codex dir, want none", candidates)
	}
}

// TestCollectSorted verifies the report order is deterministic: by kind,
// then path.
func TestCollectSorted(t *testing.T) {
	configDir := configDirFixture(t)
	write(t, filepath.Join(configDir, "plugins", "cache", "mkt", "b-orphan"), "f", "x")
	write(t, filepath.Join(configDir, "plugins", "cache", "mkt", "a-orphan"), "f", "x")
	write(t, configDir, "plugins/installed_plugins.json", `{"version": 2, "plugins": {}}`)

	skillPath := filepath.Join(configDir, "skills", "broken")
	findings := []doctor.Finding{{Check: "skill-md-missing", Path: skillPath, Message: "broken"}}

	candidates := Collect([]discovery.ConfigDir{claudeDir(configDir)}, findings)
	if len(candidates) != 3 {
		t.Fatalf("got %d candidates %+v, want 3", len(candidates), candidates)
	}
	if candidates[0].Kind != KindBrokenArtifact {
		t.Fatalf("order = %+v, want broken-artifact first (kind sort)", candidates)
	}
	if !strings.HasSuffix(candidates[1].Path, "a-orphan") || !strings.HasSuffix(candidates[2].Path, "b-orphan") {
		t.Fatalf("orphans not path-sorted: %+v", candidates[1:])
	}
}

// stubExec replaces harness.ClaudeExec (the Claude adapter's CLI executor,
// which Apply reaches through manage's helpers) for the test's lifetime,
// recording every argument list.
func stubExec(t *testing.T) *[][]string {
	t.Helper()
	var got [][]string
	orig := harness.ClaudeExec
	harness.ClaudeExec = func(args ...string) (string, error) {
		got = append(got, args)
		return "", nil
	}
	t.Cleanup(func() { harness.ClaudeExec = orig })
	return &got
}

// TestApplyRemoveDir verifies a remove-dir candidate deletes the folder from
// disk without touching the claude CLI.
func TestApplyRemoveDir(t *testing.T) {
	got := stubExec(t)
	configDir := configDirFixture(t)
	orphan := filepath.Join(configDir, "plugins", "cache", "mkt", "orphan")
	write(t, orphan, "f", "x")

	if err := Apply(Candidate{Kind: KindCacheOrphan, Path: orphan, Action: ActionRemoveDir}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("the orphan folder still exists")
	}
	if len(*got) != 0 {
		t.Fatalf("remove-dir called the claude CLI: %v", *got)
	}
}

// TestApplyDelegatesToClaude verifies the registry-owned candidates go
// through the claude CLI with the right arguments.
func TestApplyDelegatesToClaude(t *testing.T) {
	got := stubExec(t)

	if err := Apply(Candidate{Action: ActionUninstall, Arg: "p@mkt"}); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}
	if err := Apply(Candidate{Action: ActionRemoveMarketplace, Arg: "dead"}); err != nil {
		t.Fatalf("marketplace remove failed: %v", err)
	}

	want := []string{"plugin uninstall p@mkt", "plugin marketplace remove dead"}
	if len(*got) != len(want) {
		t.Fatalf("got %d claude calls %v, want %d", len(*got), *got, len(want))
	}
	for i := range want {
		if strings.Join((*got)[i], " ") != want[i] {
			t.Fatalf("call %d = %v, want %q", i, (*got)[i], want[i])
		}
	}
}

// TestActionJSON verifies actions encode as their names and round-trip.
func TestActionJSON(t *testing.T) {
	raw, err := json.Marshal(Candidate{Action: ActionUninstall})
	if err != nil || !strings.Contains(string(raw), `"action":"uninstall-plugin"`) {
		t.Fatalf("candidate marshals to %s (%v)", raw, err)
	}

	var back Candidate
	if err := json.Unmarshal(raw, &back); err != nil || back.Action != ActionUninstall {
		t.Fatalf("round-trip = %+v (%v), want uninstall back", back, err)
	}
	if err := json.Unmarshal([]byte(`{"action": "explode"}`), &back); err == nil {
		t.Fatal("an unknown action name was accepted")
	}
}

// TestHumanSize verifies the size renderer across unit boundaries.
func TestHumanSize(t *testing.T) {
	cases := map[int64]string{
		0:                    "0 B",
		512:                  "512 B",
		2048:                 "2.0 KB",
		3 * 1024 * 1024:      "3.0 MB",
		5 << 30:              "5.0 GB",
		1536 * 1024:          "1.5 MB",
		int64(1024*1024) - 1: "1024.0 KB",
	}
	for n, want := range cases {
		if got := HumanSize(n); got != want {
			t.Fatalf("HumanSize(%d) = %q, want %q", n, got, want)
		}
	}
}
