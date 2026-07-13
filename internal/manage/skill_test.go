package manage

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/scaffolding"
	"os"
	"path/filepath"
	"testing"
)

// writeSkill lays a valid skill fixture under dir/name: a SKILL.md with
// parseable frontmatter, a reference file and an executable script, so the
// copy has structure and permissions to preserve.
func writeSkill(t *testing.T, dir, name string) string {
	t.Helper()
	skill := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Join(skill, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := "---\nname: " + name + "\ndescription: \"Fixture skill\"\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "scripts", "run.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return skill
}

// configDir creates an empty harness config dir under base.
func configDir(t *testing.T, base, marker string) string {
	t.Helper()
	dir := filepath.Join(base, marker)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestInstallSkillIntoCopiesAndVerifies checks the happy path: every file
// lands, permissions survive, and the copy hashes identical to the source.
func TestInstallSkillIntoCopiesAndVerifies(t *testing.T) {
	src := writeSkill(t, t.TempDir(), "porta")
	cfg := configDir(t, t.TempDir(), ".codex")

	result := InstallSkillInto(src, cfg, false)
	if result.Status != SkillInstalled || result.Err != nil {
		t.Fatalf("install: status=%v err=%v", result.Status, result.Err)
	}
	if result.Harness != "codex" {
		t.Fatalf("harness = %q, want codex", result.Harness)
	}

	dest := filepath.Join(cfg, "skills", "porta")
	if result.Path != dest {
		t.Fatalf("path = %q, want %q", result.Path, dest)
	}
	info, err := os.Stat(filepath.Join(dest, "scripts", "run.sh"))
	if err != nil {
		t.Fatalf("script missing in the copy: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("script mode = %v, want 0755", info.Mode().Perm())
	}
	if discovery.HashDir(src) != discovery.HashDir(dest) {
		t.Fatal("copy does not hash identical to the source")
	}
}

// TestInstallSkillIntoRefusesOverwrite checks an existing skill is skipped
// without force and replaced with it.
func TestInstallSkillIntoRefusesOverwrite(t *testing.T) {
	src := writeSkill(t, t.TempDir(), "porta")
	cfg := configDir(t, t.TempDir(), ".claude")

	if r := InstallSkillInto(src, cfg, false); r.Status != SkillInstalled {
		t.Fatalf("first install: %v (%v)", r.Status, r.Err)
	}

	// A second install without force is a skip, and the copy is untouched.
	if r := InstallSkillInto(src, cfg, false); r.Status != SkillSkipped || r.Err != nil {
		t.Fatalf("re-install: status=%v err=%v, want skipped", r.Status, r.Err)
	}

	// Change the source: force must propagate the new content.
	if err := os.WriteFile(filepath.Join(src, "extra.md"), []byte("more\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := InstallSkillInto(src, cfg, true); r.Status != SkillInstalled {
		t.Fatalf("forced install: %v (%v)", r.Status, r.Err)
	}
	dest := filepath.Join(cfg, "skills", "porta")
	if _, err := os.Stat(filepath.Join(dest, "extra.md")); err != nil {
		t.Fatal("forced install did not propagate the new file")
	}
	if discovery.HashDir(src) != discovery.HashDir(dest) {
		t.Fatal("forced copy does not hash identical to the source")
	}
}

// TestInstallSkillIntoValidatesBeforeCopying checks the frontmatter gate:
// nothing is written for a non-skill or a broken SKILL.md.
func TestInstallSkillIntoValidatesBeforeCopying(t *testing.T) {
	cfg := configDir(t, t.TempDir(), ".claude")

	// No SKILL.md at all.
	empty := filepath.Join(t.TempDir(), "not-a-skill")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if r := InstallSkillInto(empty, cfg, false); r.Status != SkillFailed {
		t.Fatalf("dir without SKILL.md: %v, want failed", r.Status)
	}

	// A SKILL.md whose frontmatter never closes.
	broken := filepath.Join(t.TempDir(), "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "SKILL.md"), []byte("---\nname: broken\nno closing fence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := InstallSkillInto(broken, cfg, false); r.Status != SkillFailed {
		t.Fatalf("broken frontmatter: %v, want failed", r.Status)
	}

	if _, err := os.Stat(filepath.Join(cfg, "skills")); !os.IsNotExist(err) {
		t.Fatal("a rejected install still wrote into the config dir")
	}
}

// TestInstallSkillIntoGuards checks the structural refusals: a destination
// that is no config dir, a missing config dir, and copying onto itself.
func TestInstallSkillIntoGuards(t *testing.T) {
	src := writeSkill(t, t.TempDir(), "porta")

	if r := InstallSkillInto(src, t.TempDir(), false); r.Status != SkillFailed {
		t.Fatalf("non config dir: %v, want failed", r.Status)
	}

	missing := filepath.Join(t.TempDir(), ".claude")
	if r := InstallSkillInto(src, missing, false); r.Status != SkillFailed {
		t.Fatalf("missing config dir: %v, want failed", r.Status)
	}

	// A skill already inside a config dir, installed into its own home.
	base := t.TempDir()
	cfg := configDir(t, base, ".claude")
	inPlace := writeSkill(t, filepath.Join(cfg, "skills"), "self")
	if r := InstallSkillInto(inPlace, cfg, true); r.Status != SkillFailed {
		t.Fatalf("self copy: %v (%v), want failed", r.Status, r.Err)
	}
	if _, err := os.Stat(inPlace); err != nil {
		t.Fatal("the self copy destroyed the source")
	}
}

// TestInstallSkillIntoSkipsSymlinks checks a symlink in the source is not
// followed nor copied, and the hash verification still passes.
func TestInstallSkillIntoSkipsSymlinks(t *testing.T) {
	src := writeSkill(t, t.TempDir(), "porta")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}

	cfg := configDir(t, t.TempDir(), ".opencode")
	result := InstallSkillInto(src, cfg, false)
	if result.Status != SkillInstalled {
		t.Fatalf("install with symlink: %v (%v)", result.Status, result.Err)
	}
	if _, err := os.Lstat(filepath.Join(result.Path, "link.txt")); !os.IsNotExist(err) {
		t.Fatal("the symlink was copied")
	}
}

// TestInstallSkillScopes checks the single-harness entry point resolves the
// scope base and demands the config dir exists.
func TestInstallSkillScopes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := writeSkill(t, t.TempDir(), "porta")

	// No ~/.claude yet: the install must refuse.
	if _, err := InstallSkill(src, scaffolding.Claude, scaffolding.Local, false); err == nil {
		t.Fatal("install into a missing config dir did not error")
	}

	configDir(t, home, ".claude")
	result, err := InstallSkill(src, scaffolding.Claude, scaffolding.Local, false)
	if err != nil || result.Status != SkillInstalled {
		t.Fatalf("install: err=%v status=%v", err, result.Status)
	}
	if want := filepath.Join(home, ".claude", "skills", "porta"); result.Path != want {
		t.Fatalf("path = %q, want %q", result.Path, want)
	}
}

// TestInstallSkillAllFansOut checks the fan-out targets exactly the config
// dirs present at the scope and reports each one individually.
func TestInstallSkillAllFansOut(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := writeSkill(t, t.TempDir(), "porta")

	// No config dirs at all: an error, not an empty success.
	if _, err := InstallSkillAll(src, scaffolding.Local, false); err == nil {
		t.Fatal("fan-out over nothing did not error")
	}

	claude := configDir(t, home, ".claude")
	configDir(t, home, ".codex")
	// Pre-place the skill in .claude so that target reports a skip.
	if r := InstallSkillInto(src, claude, false); r.Status != SkillInstalled {
		t.Fatalf("seed install: %v", r.Status)
	}

	results, err := InstallSkillAll(src, scaffolding.Local, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (claude and codex)", len(results))
	}
	if results[0].Harness != "claude" || results[0].Status != SkillSkipped {
		t.Fatalf("claude target: %s %v, want skipped", results[0].Harness, results[0].Status)
	}
	if results[1].Harness != "codex" || results[1].Status != SkillInstalled {
		t.Fatalf("codex target: %s %v, want installed", results[1].Harness, results[1].Status)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "porta", "SKILL.md")); err != nil {
		t.Fatal("fan-out did not land the skill in .codex")
	}
}
