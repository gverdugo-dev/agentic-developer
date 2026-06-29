package cli

import (
	"agentic-developer/internal/scaffolding"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

// autoDetect is the harness select option that leaves detection to the engine.
// Its value is the empty string, which newScaffoldRequest reads as "auto".
const autoDetect = ""

// interactiveAvailable reports whether an interactive form can run: both stdin
// and stdout must be real terminals. When either is a pipe or file (a script or
// CI), the caller falls back to the non-interactive usage error so behavior
// stays unchanged for non-terminal callers.
func interactiveAvailable() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// runNewForm prompts for the fields of a `new` invocation and returns the parsed
// request. The flag values seed the defaults, so `adev new --scope local` opens
// the form with that scope preselected. The collected strings go through the
// same newScaffoldRequest validation as the non-interactive path.
func runNewForm(defaultScope, defaultHarness string, defaultForce bool) (scaffoldRequest, error) {
	artifact := scaffolding.Artifacts[scaffolding.Skill]
	name := ""
	scope := defaultScope
	harness := defaultHarness
	force := defaultForce

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Artifact").
				Description("What to scaffold").
				Options(artifactOptions()...).
				Value(&artifact),

			huh.NewInput().
				Title("Name").
				Description("kebab-case, no slashes").
				Placeholder("my-new-skill").
				Value(&name).
				Validate(validateArtifactName),

			huh.NewSelect[string]().
				Title("Scope").
				Description("Where it lives").
				Options(scopeOptions()...).
				Value(&scope),

			huh.NewSelect[string]().
				Title("Harness").
				Description("Target agent (auto-detect from the folders by default)").
				Options(harnessOptions()...).
				Value(&harness),

			huh.NewConfirm().
				Title("Overwrite if it already exists?").
				Value(&force),
		),
	).WithTheme(brandTheme())

	if err := form.Run(); err != nil {
		return scaffoldRequest{}, err
	}

	return newScaffoldRequest(scaffolding.New, artifact, strings.TrimSpace(name), scope, harness, force)
}

// artifactOptions lists the scaffoldable artifacts in a stable order.
func artifactOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption(scaffolding.Artifacts[scaffolding.Skill], scaffolding.Artifacts[scaffolding.Skill]),
		huh.NewOption(scaffolding.Artifacts[scaffolding.Plugin], scaffolding.Artifacts[scaffolding.Plugin]),
		huh.NewOption(scaffolding.Artifacts[scaffolding.PluginMarketplace], scaffolding.Artifacts[scaffolding.PluginMarketplace]),
	}
}

// scopeOptions lists the scopes in a stable order.
func scopeOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("project (current dir)", scaffolding.Scopes[scaffolding.Project]),
		huh.NewOption("local (home)", scaffolding.Scopes[scaffolding.Local]),
	}
}

// harnessOptions lists auto-detect first, then each harness in a stable order.
func harnessOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("auto-detect", autoDetect),
		huh.NewOption(scaffolding.AIHarnesses[scaffolding.Claude], scaffolding.AIHarnesses[scaffolding.Claude]),
		huh.NewOption(scaffolding.AIHarnesses[scaffolding.Codex], scaffolding.AIHarnesses[scaffolding.Codex]),
		huh.NewOption(scaffolding.AIHarnesses[scaffolding.Opencode], scaffolding.AIHarnesses[scaffolding.Opencode]),
	}
}

// validateArtifactName rejects names the scaffolding engine would refuse later,
// so the form catches them before submission. It mirrors RemoveConfig's rules.
func validateArtifactName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if strings.ContainsAny(name, `/\`) || name == ".." {
		return fmt.Errorf("name cannot contain slashes")
	}
	return nil
}

// brandTheme tints huh's base theme with the adev brand palette so the form
// matches the styled output: blue for focus, teal for the selected option.
func brandTheme() *huh.Theme {
	t := huh.ThemeBase()
	t.Focused.Title = t.Focused.Title.Foreground(brandBlue).Bold(true)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(brandBlue)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(brandTeal)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Background(brandBlue)
	return t
}
