package adevfile

import (
	"agentic-developer/internal/discovery"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// write writes one file under base, failing the test on error.
func write(t *testing.T, base, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(base, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureHome builds a home with a registry-backed ~/.claude: one skill, one
// installed plugin (enabled) and one github marketplace.
func fixtureHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkdirs(t, home, ".claude/skills/home-skill", ".claude/plugins")

	claude := filepath.Join(home, ".claude")
	write(t, claude, "plugins/installed_plugins.json",
		`{"plugins": {"tool@some-mkt": [{"version": "1.0.0"}]}}`)
	write(t, claude, "plugins/known_marketplaces.json",
		`{"some-mkt": {"source": {"source": "github", "repo": "acme/some-mkt"}}}`)
	write(t, claude, "settings.json", `{"enabledPlugins": {"tool@some-mkt": true}}`)
	return home
}

// TestFromScan verifies the manifest built from a discovered reality: user
// scope from the home config dirs, project scope from the scan root's own
// config dirs, deeper config dirs skipped.
func TestFromScan(t *testing.T) {
	fixtureHome(t)

	root := t.TempDir()
	mkdirs(t, root, ".codex/skills/proj-skill", "nested/other/.claude/skills/foreign")

	dirs, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	file, skipped, err := FromScan(dirs, root)
	if err != nil {
		t.Fatal(err)
	}

	claude := file.Harnesses["claude"]
	if claude.User == nil || claude.Project != nil {
		t.Fatalf("claude scopes = %+v, want user only", claude)
	}
	if got := strings.Join(claude.User.Skills, ","); got != "home-skill" {
		t.Fatalf("claude user skills = %q", got)
	}
	if got := strings.Join(claude.User.Plugins, ","); got != "tool@some-mkt" {
		t.Fatalf("claude user plugins = %q", got)
	}
	wantMkt := []Marketplace{{Name: "some-mkt", Source: "acme/some-mkt"}}
	if !reflect.DeepEqual(claude.User.Marketplaces, wantMkt) {
		t.Fatalf("claude user marketplaces = %+v, want %+v", claude.User.Marketplaces, wantMkt)
	}

	codex := file.Harnesses["codex"]
	if codex.Project == nil || codex.User != nil {
		t.Fatalf("codex scopes = %+v, want project only", codex)
	}
	if got := strings.Join(codex.Project.Skills, ","); got != "proj-skill" {
		t.Fatalf("codex project skills = %q", got)
	}

	wantSkipped := filepath.Join(root, "nested", "other", ".claude")
	if len(skipped) != 1 || skipped[0] != wantSkipped {
		t.Fatalf("skipped = %v, want [%s]", skipped, wantSkipped)
	}
}

// TestExportRoundTrip verifies the export contract: encoding the manifest
// built from a scan and parsing it back yields the same state, and that
// state syncs clean against the reality it was taken from.
func TestExportRoundTrip(t *testing.T) {
	fixtureHome(t)
	root := t.TempDir()
	mkdirs(t, root, ".opencode/skills/oc-skill")

	dirs, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	exported, _, err := FromScan(dirs, root)
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := exported.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("exported manifest does not parse: %v\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(exported, parsed) {
		t.Fatalf("round trip drifted:\n%+v\nvs\n%+v", exported, parsed)
	}

	plan, err := Diff(parsed, dirs, root)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.InSync() || plan.Extra() != 0 {
		t.Fatalf("exported manifest is not in sync with its own reality: %+v", plan.Items)
	}
}

// TestAddableSource verifies the source label round trip for every source
// kind discovery reports.
func TestAddableSource(t *testing.T) {
	cases := map[string]string{
		"github acme/some-mkt":      "acme/some-mkt",
		"git https://x.test/r.git":  "https://x.test/r.git",
		"directory /tmp/mkt":        "/tmp/mkt",
		"":                          "",
		"github":                    "",
		"mystery /where/is/this":    "",
		"directory /with space/mkt": "/with space/mkt",
	}
	for label, want := range cases {
		if got := addableSource(label); got != want {
			t.Fatalf("addableSource(%q) = %q, want %q", label, got, want)
		}
	}
}
