package cli

import (
	"agentic-developer/internal/scaffolding"
	"agentic-developer/skills"
	"path/filepath"
)

// setupCmd implements the setup command: it installs adev's own bundled skills
// into a harness under the user's home.
type setupCmd struct{}

// Name returns the command's CLI word.
func (setupCmd) Name() string { return "setup" }

// Synopsis returns the one-line help for the command.
func (setupCmd) Synopsis() string {
	return "install adev's bundled skills into a harness (home)"
}

// Run installs adev's own bundled skills into the user's harness so the agent
// learns how to use the tool. It always targets the user's config (home), never
// the project. The harness is taken from the optional positional argument, or
// auto-detected from the user's home when omitted.
//
// args are the arguments after "setup", i.e. an optional harness name.
func (setupCmd) Run(args []string) error {
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

	printSuccess("installed %d adev skill(s) for %s into %s",
		len(installed), accent(scaffolding.AIHarnesses[harness]), muted(destDir))
	for _, name := range installed {
		printInfo("  %s %s", muted("-"), accent(name))
	}
	return nil
}
