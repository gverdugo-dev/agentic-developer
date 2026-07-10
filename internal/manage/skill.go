package manage

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/harness"
	"agentic-developer/internal/scaffolding"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// This file installs (copies) a skill directory into a harness's skills
// container. Skills are the one artifact portable across every supported
// harness (they all load the shared SKILL.md standard), so the copier lives
// in the shared mutation layer: the TUI's "c" (copy to harness) and
// `adev skill install` both run through it.

// SkillInstallStatus classifies the outcome of one install target. The zero
// value is failure, so a result is never accidentally reported as a success.
type SkillInstallStatus int

// The outcomes, per target.
const (
	// SkillFailed: the copy did not happen (or did not verify).
	SkillFailed SkillInstallStatus = iota
	// SkillSkipped: the target already holds a skill with that name and
	// force was off. Skipping is a refusal, not an error: fan-out targets
	// report it individually.
	SkillSkipped
	// SkillInstalled: the skill was copied and the copy verified.
	SkillInstalled
)

// String names the status for display and JSON output.
func (s SkillInstallStatus) String() string {
	switch s {
	case SkillSkipped:
		return "skipped"
	case SkillInstalled:
		return "installed"
	default:
		return "failed"
	}
}

// MarshalText makes the status encode as its name in JSON.
func (s SkillInstallStatus) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// SkillInstall is the result of installing one skill into one config dir.
type SkillInstall struct {
	// Harness is the display name of the target's harness ("claude").
	Harness string
	// ConfigDir is the target config dir, absolute.
	ConfigDir string
	// Path is the destination skill dir, "" when it could not be resolved.
	Path string
	// Status says what happened; Err carries the failure when SkillFailed.
	Status SkillInstallStatus
	Err    error
}

// ValidateSkillDir checks that srcDir holds an installable skill, before any
// copying: it must be a directory carrying a SKILL.md whose frontmatter
// parses. The check uses the exact reading discovery uses, so anything adev
// lists as a skill validates here too.
func ValidateSkillDir(srcDir string) error {
	info, err := os.Stat(srcDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", srcDir)
	}

	manifest := filepath.Join(srcDir, "SKILL.md")
	if _, err := os.Stat(manifest); err != nil {
		return fmt.Errorf("%q has no SKILL.md, so it is not a skill", srcDir)
	}
	if discovery.ParseFrontmatter(manifest) == nil {
		return fmt.Errorf("the SKILL.md of %q has no parsable frontmatter block", srcDir)
	}
	return nil
}

// InstallSkillInto copies the skill at srcDir into configDir's skills
// container, keeping the skill's folder name. The source is validated before
// anything is written, an existing skill of the same name is only replaced
// with force, and the landed copy is verified against the source's content
// hash (a failed verification removes the partial copy). The outcome is
// always reported as a SkillInstall, never a bare error, so fan-out callers
// can report every target individually.
func InstallSkillInto(srcDir, configDir string, force bool) SkillInstall {
	result := SkillInstall{ConfigDir: configDir}
	fail := func(err error) SkillInstall {
		result.Status = SkillFailed
		result.Err = err
		return result
	}

	src, err := filepath.Abs(srcDir)
	if err != nil {
		return fail(err)
	}
	cfg, err := filepath.Abs(configDir)
	if err != nil {
		return fail(err)
	}
	result.ConfigDir = cfg

	ad, ok := harness.ForMarker(filepath.Base(cfg))
	if !ok {
		return fail(fmt.Errorf("%q is not a harness config dir (.claude, .codex, .opencode)", cfg))
	}
	result.Harness = ad.Name()
	container, ok := skillContainer(ad)
	if !ok {
		return fail(fmt.Errorf("harness %s has no skills folder", ad.Name()))
	}
	if !isDirPath(cfg) {
		return fail(fmt.Errorf("config dir %q does not exist", cfg))
	}

	if err := ValidateSkillDir(src); err != nil {
		return fail(err)
	}

	dest := filepath.Join(cfg, container, filepath.Base(src))
	result.Path = dest
	if dest == src {
		return fail(fmt.Errorf("source and destination are the same directory"))
	}

	if _, err := os.Lstat(dest); err == nil {
		if !force {
			result.Status = SkillSkipped
			return result
		}
		// The forced replacement goes through the guarded delete, so even a
		// crafted destination can never remove something outside a config dir.
		if err := DeleteArtifact(dest); err != nil {
			return fail(err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fail(err)
	}
	if err := copyTree(src, dest); err != nil {
		os.RemoveAll(dest) // never leave a half-copied skill behind
		return fail(err)
	}

	// The copy must land byte-identical: the same content hash that powers
	// duplicate detection is the cheap proof of that.
	if srcHash := discovery.HashDir(src); srcHash == "" || srcHash != discovery.HashDir(dest) {
		os.RemoveAll(dest)
		return fail(fmt.Errorf("copy verification failed: content hashes differ"))
	}

	result.Status = SkillInstalled
	return result
}

// InstallSkill installs the skill at srcDir into one harness at the given
// scope (Project = the current dir, Local = the user's home). The harness's
// config dir must already exist there: installing is a management operation
// on a harness the user has, never the way a config dir first appears.
func InstallSkill(srcDir string, h scaffolding.AIHarness, scope scaffolding.Scope, force bool) (SkillInstall, error) {
	base, err := scopeBase(scope)
	if err != nil {
		return SkillInstall{}, err
	}

	configDir := filepath.Join(base, scaffolding.MarkerFor(h))
	if !isDirPath(configDir) {
		return SkillInstall{}, fmt.Errorf("no %s config dir in %s: scaffold something into it first, or pick another harness", scaffolding.MarkerFor(h), base)
	}
	return InstallSkillInto(srcDir, configDir, force), nil
}

// InstallSkillAll fans the install out to every harness whose config dir is
// present at the given scope, returning one result per target in harness
// priority order. No harness at all is an error; per-target problems are
// reported in the results instead.
func InstallSkillAll(srcDir string, scope scaffolding.Scope, force bool) ([]SkillInstall, error) {
	base, err := scopeBase(scope)
	if err != nil {
		return nil, err
	}

	var results []SkillInstall
	for _, ad := range harness.All() {
		configDir := filepath.Join(base, ad.Marker())
		if !isDirPath(configDir) {
			continue
		}
		results = append(results, InstallSkillInto(srcDir, configDir, force))
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no harness config dirs found in %s", base)
	}
	return results, nil
}

// copyTree copies every regular file under src into dest, preserving each
// file's permission bits, in the deterministic lexical order of WalkDir.
// Symlinks and other irregular files are skipped on purpose: following a
// link out of the skill dir is exactly the kind of surprise an installer
// must not have, and the content hash that verifies the copy skips them the
// same way, so a skipped link never fails the verification.
func copyTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !d.Type().IsRegular() {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode().Perm())
	})
}

// skillContainer returns the skills folder of one adapter, relative to its
// config dir.
func skillContainer(ad harness.Adapter) (string, bool) {
	for _, c := range ad.Containers() {
		if c.Kind == harness.KindSkill {
			return c.Dir, true
		}
	}
	return "", false
}

// scopeBase resolves a scope to the absolute base dir its config dirs sit in.
func scopeBase(scope scaffolding.Scope) (string, error) {
	base, err := scaffolding.ScopeBaseDir(scope)
	if err != nil {
		return "", err
	}
	return filepath.Abs(base)
}

// isDirPath reports whether path exists and is a directory.
func isDirPath(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
