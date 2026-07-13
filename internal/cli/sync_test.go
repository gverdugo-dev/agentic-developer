package cli

import (
	"agentic-developer/internal/adevfile"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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

// writeFile writes one file under base, failing the test on error.
func writeFile(t *testing.T, base, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(base, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// captureStdoutErr runs fn with os.Stdout redirected and returns what it
// wrote plus fn's error, for commands whose non-zero verdict is part of the
// contract (sync's diff-style exit).
func captureStdoutErr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fnErr := fn()
	w.Close()
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(out), fnErr
}

// syncFixture builds an isolated home (~/.claude with one skill and a
// registry-installed plugin) and an empty project dir the test runs in.
func syncFixture(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkdirs(t, home, ".claude/skills/home-skill", ".claude/plugins")
	claude := filepath.Join(home, ".claude")
	writeFile(t, claude, "plugins/installed_plugins.json",
		`{"plugins": {"tool@some-mkt": [{"version": "1.0.0"}]}}`)
	writeFile(t, claude, "plugins/known_marketplaces.json",
		`{"some-mkt": {"source": {"source": "github", "repo": "acme/some-mkt"}}}`)

	project := t.TempDir()
	t.Chdir(project)
}

// TestSyncDryRunJSON verifies the dry run: the JSON plan on stdout, no
// claude CLI call, and the diff-style non-zero verdict when items are
// missing.
func TestSyncDryRunJSON(t *testing.T) {
	syncFixture(t)
	called := stubExec(t, "")
	writeFile(t, ".", "adevfile.json", `{
	  "version": 1,
	  "harnesses": {"claude": {"user": {
	    "skills": ["home-skill"],
	    "plugins": ["tool@some-mkt", "wanted@some-mkt"]
	  }}}
	}`)

	out, err := captureStdoutErr(t, func() error {
		return syncCmd{}.Run([]string{"--json"})
	})
	if err == nil || !strings.Contains(err.Error(), "1 item(s) missing") {
		t.Fatalf("dry run with missing items returned %v, want the missing verdict", err)
	}
	if len(*called) != 0 {
		t.Fatalf("dry run called the claude CLI: %v", *called)
	}

	var report struct {
		InSync  bool
		Missing int
		Extra   int
		Items   []adevfile.Item
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out)
	}
	if report.InSync || report.Missing != 1 {
		t.Fatalf("report = %+v, want 1 missing", report)
	}
	if report.Extra != 1 { // the registered marketplace is undeclared
		t.Fatalf("report = %+v, want 1 extra (the undeclared marketplace)", report)
	}
}

// TestSyncInSyncExitsClean verifies a satisfied manifest exits 0.
func TestSyncInSyncExitsClean(t *testing.T) {
	syncFixture(t)
	stubExec(t, "")
	writeFile(t, ".", "adevfile.json", `{
	  "version": 1,
	  "harnesses": {"claude": {"user": {"skills": ["home-skill"]}}}
	}`)

	if _, err := captureStdoutErr(t, func() error {
		return syncCmd{}.Run(nil)
	}); err != nil {
		t.Fatalf("in-sync dry run failed: %v", err)
	}
}

// TestSyncApply verifies --apply executes exactly the installable missing
// items: plugins and marketplaces through the claude CLI, skills through the
// wired InstallSkill seam (stubbed here; the real copier has its own test).
func TestSyncApply(t *testing.T) {
	syncFixture(t)
	called := stubExec(t, "ok")

	var seamCalls []string
	orig := adevfile.InstallSkill
	adevfile.InstallSkill = func(name, harnessLabel, scope string) (string, error) {
		seamCalls = append(seamCalls, name+" into "+harnessLabel+"/"+scope)
		return "copied", nil
	}
	t.Cleanup(func() { adevfile.InstallSkill = orig })

	writeFile(t, ".", "manifest.json", `{
	  "version": 1,
	  "harnesses": {"claude": {"user": {
	    "skills": ["home-skill", "missing-skill"],
	    "plugins": ["tool@some-mkt", "wanted@some-mkt"],
	    "marketplaces": [{"name": "new-mkt", "source": "acme/new-mkt"}]
	  }}}
	}`)

	out, err := captureStdoutErr(t, func() error {
		return syncCmd{}.Run([]string{"manifest.json", "--apply", "--json"})
	})
	if err != nil {
		t.Fatalf("apply failed: %v (%q)", err, out)
	}

	want := []string{
		"plugin install wanted@some-mkt",
		"plugin marketplace add acme/new-mkt",
	}
	if len(*called) != len(want) {
		t.Fatalf("claude called %d times, want %d: %v", len(*called), len(want), *called)
	}
	for i := range want {
		if strings.Join((*called)[i], " ") != want[i] {
			t.Fatalf("call %d = %v, want %q", i, (*called)[i], want[i])
		}
	}

	var results []struct {
		adevfile.Item
		Applied bool
		Output  string
	}
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out)
	}
	if len(results) != 3 {
		t.Fatalf("results = %+v, want 3 applied items", results)
	}
	for _, r := range results {
		if !r.Applied {
			t.Fatalf("item not applied: %+v", r)
		}
	}
	// The missing skill went through the wired seam, not the claude CLI.
	if len(seamCalls) != 1 || seamCalls[0] != "missing-skill into claude/user" {
		t.Fatalf("seam calls = %v", seamCalls)
	}
}

// TestSyncApplyInstallsSkill exercises the real wired seam end to end: the
// manifest wants a skill in claude/user that only exists in the home codex
// dir, and --apply copies it across harnesses by its discovered name.
func TestSyncApplyInstallsSkill(t *testing.T) {
	syncFixture(t)
	called := stubExec(t, "")

	home := os.Getenv("HOME")
	mkdirs(t, home, ".codex/skills/portable-skill")
	writeFile(t, home, ".codex/skills/portable-skill/SKILL.md",
		"---\nname: portable-skill\ndescription: fixture\n---\n\nbody\n")
	writeFile(t, ".", "adevfile.json", `{
	  "version": 1,
	  "harnesses": {"claude": {"user": {"skills": ["home-skill", "portable-skill"]}}}
	}`)

	if out, err := captureStdoutErr(t, func() error {
		return syncCmd{}.Run([]string{"--apply"})
	}); err != nil {
		t.Fatalf("apply failed: %v (%q)", err, out)
	}
	if len(*called) != 0 {
		t.Fatalf("a skill install reached the claude CLI: %v", *called)
	}

	installed := filepath.Join(home, ".claude/skills/portable-skill/SKILL.md")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("the skill did not land in ~/.claude: %v", err)
	}

	// A second sync sees the copy: the manifest is now satisfied.
	if _, err := captureStdoutErr(t, func() error {
		return syncCmd{}.Run(nil)
	}); err != nil {
		t.Fatalf("re-sync after apply not clean: %v", err)
	}
}

// TestSyncRejectsBadManifest verifies a malformed manifest fails before any
// scan or CLI call, with the parser's spot-naming error.
func TestSyncRejectsBadManifest(t *testing.T) {
	syncFixture(t)
	called := stubExec(t, "")
	writeFile(t, ".", "adevfile.json", `{"version": 1, "harnesses": {"cursor": {}}}`)

	_, err := captureStdoutErr(t, func() error {
		return syncCmd{}.Run([]string{"--apply"})
	})
	if err == nil || !strings.Contains(err.Error(), "harnesses.cursor") {
		t.Fatalf("bad manifest returned %v, want the validation error", err)
	}
	if len(*called) != 0 {
		t.Fatalf("bad manifest reached the claude CLI: %v", *called)
	}
}

// TestExportStdout verifies `adev export` writes a manifest to stdout that
// parses and describes the fixture reality.
func TestExportStdout(t *testing.T) {
	syncFixture(t)
	mkdirs(t, ".", ".codex/skills/proj-skill")

	out, err := captureStdoutErr(t, func() error {
		return exportCmd{}.Run(nil)
	})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	manifest, err := adevfile.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("exported manifest does not parse: %v\n%s", err, out)
	}
	claude := manifest.Harnesses["claude"]
	if claude.User == nil || len(claude.User.Skills) != 1 || claude.User.Skills[0] != "home-skill" {
		t.Fatalf("claude user = %+v", claude.User)
	}
	if len(claude.User.Plugins) != 1 || claude.User.Plugins[0] != "tool@some-mkt" {
		t.Fatalf("claude user plugins = %+v", claude.User.Plugins)
	}
	codex := manifest.Harnesses["codex"]
	if codex.Project == nil || len(codex.Project.Skills) != 1 || codex.Project.Skills[0] != "proj-skill" {
		t.Fatalf("codex project = %+v", codex.Project)
	}
}

// TestExportToFileThenSync verifies the export-then-sync loop: the written
// adevfile.json parses and syncs clean against the state it was taken from.
func TestExportToFileThenSync(t *testing.T) {
	syncFixture(t)

	if _, err := captureStdoutErr(t, func() error {
		return exportCmd{}.Run([]string{"adevfile.json"})
	}); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if _, err := captureStdoutErr(t, func() error {
		return syncCmd{}.Run(nil)
	}); err != nil {
		t.Fatalf("sync against the exported manifest failed: %v", err)
	}
}
