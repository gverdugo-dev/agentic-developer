package cli

import (
	"agentic-developer/internal/adevfile"
	"agentic-developer/internal/manage"
	"agentic-developer/internal/scaffolding"
	"fmt"
)

// This file is the composition root of the T6/T7 seam. The adevfile sync
// engine asks for a missing skill by its manifest name, and manage owns the
// cross-harness copier; neither package may import the other's concern, so
// the cli package, which already sees both, wires them together. With the
// seam wired, `adev sync --apply` installs missing skills the same way
// `adev skill install <name>` does, instead of reporting them as manual.
func init() {
	adevfile.InstallSkill = installSkillByName
}

// installSkillByName satisfies the adevfile.InstallSkill seam: it resolves
// the manifest skill name to a discovered source dir (the exact resolution
// `adev skill install <name>` uses, ambiguity included), then copies it into
// the harness config dir at the given scope through manage. Sync only
// applies missing items, so an existing destination is never overwritten:
// force stays off.
func installSkillByName(name, harnessLabel, scope string) (string, error) {
	h, err := scaffolding.ParseHarness(harnessLabel)
	if err != nil {
		return "", err
	}
	sc, err := parseSkillScope(scope)
	if err != nil {
		return "", err
	}
	srcDir, err := resolveSkillSource(name)
	if err != nil {
		return "", err
	}

	result, err := manage.InstallSkill(srcDir, h, sc, false)
	if err != nil {
		return "", err
	}
	switch result.Status {
	case manage.SkillInstalled:
		return fmt.Sprintf("installed %s into %s", name, result.ConfigDir), nil
	case manage.SkillSkipped:
		return "", fmt.Errorf("%s already exists in %s", name, result.ConfigDir)
	default:
		return "", result.Err
	}
}
