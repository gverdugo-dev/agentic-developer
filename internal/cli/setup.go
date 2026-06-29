package cli

import (
	"agentic-developer/internal/scaffolding"
	"agentic-developer/skills"
	"log/slog"
	"path/filepath"
)

// runSetup installs adev's own bundled skills into the user's harness so the
// agent learns how to use the tool. It always targets the user's config (home),
// never the project. The harness is taken from the optional argument, or
// auto-detected from the user's home when omitted.
//
// args are the arguments after "setup", i.e. an optional harness name.
func runSetup(args []string) error {
	home, err := scaffolding.ScopeBaseDir(scaffolding.Local)
	if err != nil {
		return err
	}

	var harness scaffolding.AIHarness
	if len(args) >= 1 {
		harness, err = scaffolding.ParseHarness(args[0])
	} else {
		harness, err = scaffolding.DetectAIHarness(home)
	}
	if err != nil {
		return err
	}

	// Skills always land in the harness skills dir under home, e.g.
	// ~/.claude/skills.
	destDir := filepath.Join(home, scaffolding.PlacementPrefix(harness, scaffolding.Skill))

	installed, err := skills.Install(destDir)
	if err != nil {
		return err
	}

	slog.Info("adev skills installed",
		"harness", scaffolding.AIHarnesses[harness],
		"dir", destDir,
		"skills", installed,
	)
	return nil
}
