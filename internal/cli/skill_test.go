package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkillFixture lays a valid skill under dir/name.
func writeSkillFixture(t *testing.T, dir, name string) string {
	t.Helper()
	skill := filepath.Join(dir, name)
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "---\nname: " + name + "\ndescription: \"Fixture skill\"\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return skill
}

// skillInstallReport mirrors the command's JSON contract.
type skillInstallReport struct {
	Skill     string `json:"skill"`
	Harness   string `json:"harness"`
	ConfigDir string `json:"configDir"`
	Path      string `json:"path"`
	Status    string `json:"status"`
	Error     string `json:"error"`
}

// TestSkillInstallAllJSON drives `adev skill install --harness all` over a
// home with two harness config dirs and checks the per-target JSON report.
func TestSkillInstallAllJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, marker := range []string{".claude", ".codex"} {
		if err := os.MkdirAll(filepath.Join(home, marker), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	src := writeSkillFixture(t, t.TempDir(), "porta")

	out := captureStdout(t, func() error {
		return skillCmd{}.Run([]string{"install", src, "--harness", "all", "--json"})
	})

	var reports []skillInstallReport
	if err := json.Unmarshal([]byte(out), &reports); err != nil {
		t.Fatalf("bad JSON %q: %v", out, err)
	}
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want 2", len(reports))
	}
	for i, want := range []string{"claude", "codex"} {
		if reports[i].Harness != want || reports[i].Status != "installed" {
			t.Fatalf("report %d = %+v, want installed into %s", i, reports[i], want)
		}
		if _, err := os.Stat(filepath.Join(reports[i].Path, "SKILL.md")); err != nil {
			t.Fatalf("skill missing on disk at %s: %v", reports[i].Path, err)
		}
	}
}

// TestSkillInstallRefusesExisting checks the overwrite refusal surfaces as a
// nonzero exit until --force is passed.
func TestSkillInstallRefusesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := writeSkillFixture(t, t.TempDir(), "porta")

	if err := (skillCmd{}).Run([]string{"install", src, "--harness", "claude"}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	err := (skillCmd{}).Run([]string{"install", src, "--harness", "claude"})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("re-install: err = %v, want the force hint", err)
	}
	if err := (skillCmd{}).Run([]string{"install", src, "--harness", "claude", "--force"}); err != nil {
		t.Fatalf("forced install: %v", err)
	}
}

// TestSkillInstallResolvesDiscoveredName checks the positional accepts the
// name of a skill discovered under the current dir, and that a name living
// in two places is refused with the candidate paths listed.
func TestSkillInstallResolvesDiscoveredName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	work := t.TempDir()
	writeSkillFixture(t, filepath.Join(work, ".claude", "skills"), "porta")
	t.Chdir(work)

	if err := (skillCmd{}).Run([]string{"install", "porta", "--harness", "codex"}); err != nil {
		t.Fatalf("install by name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "porta", "SKILL.md")); err != nil {
		t.Fatal("the discovered skill did not land in ~/.codex")
	}

	// The same name in a second location makes it ambiguous (the discovery
	// also resurfaces the copy just installed into ~/.codex).
	writeSkillFixture(t, filepath.Join(work, "sub", ".claude", "skills"), "porta")
	err := (skillCmd{}).Run([]string{"install", "porta", "--harness", "codex", "--force"})
	if err == nil || !strings.Contains(err.Error(), "places:") || !strings.Contains(err.Error(), "pass the path") {
		t.Fatalf("ambiguous name: err = %v, want the candidates listed", err)
	}

	// A name nothing matches names the real problem.
	err = (skillCmd{}).Run([]string{"install", "no-such-skill", "--harness", "codex"})
	if err == nil || !strings.Contains(err.Error(), "neither a directory nor a discovered skill") {
		t.Fatalf("unknown name: err = %v", err)
	}
}

// TestSkillInstallScopeAndFlagValidation checks the flag surface: bad scope,
// bad harness word, bad action.
func TestSkillInstallScopeAndFlagValidation(t *testing.T) {
	src := writeSkillFixture(t, t.TempDir(), "porta")

	if err := (skillCmd{}).Run([]string{"install", src, "--scope", "global"}); err == nil {
		t.Fatal("bad scope accepted")
	}
	if err := (skillCmd{}).Run([]string{"install", src, "--harness", "cursor"}); err == nil {
		t.Fatal("bad harness accepted")
	}
	if err := (skillCmd{}).Run([]string{"uninstall", src}); err == nil {
		t.Fatal("bad action accepted")
	}
}
