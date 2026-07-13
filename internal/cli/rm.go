package cli

import (
	"agentic-developer/internal/manage"
	"agentic-developer/internal/scaffolding"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"
)

// rmCmd implements the rm command: delete a discovered artifact (a skill, a
// folder plugin, a folder marketplace) or a whole config dir, by absolute
// path. The CLI face of the TUI's "d". Deleting a whole config dir asks the
// user to type its name, the same brake the TUI applies.
type rmCmd struct{}

// Name returns the command's CLI word.
func (rmCmd) Name() string { return "rm" }

// Synopsis returns the one-line help for the command.
func (rmCmd) Synopsis() string {
	return "delete an artifact or a whole config dir by absolute path"
}

// Run parses the path positional and the --yes flag, confirms, and deletes.
func (c rmCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev rm", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev rm <absolute-path> [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\nOnly harness config dirs (.claude, .codex, .opencode) and folders")
		fmt.Fprintln(fs.Output(), "inside them can be deleted.")
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		fs.Usage()
		return fmt.Errorf("provide exactly one absolute path")
	}
	path := filepath.Clean(positional[0])

	if !*yes {
		if !interactiveAvailable() {
			return fmt.Errorf("no terminal to confirm on; pass --yes to delete %q", path)
		}
		confirmed, err := confirmDeletion(path)
		if err != nil {
			return err
		}
		if !confirmed {
			printInfo("aborted")
			return nil
		}
	}

	if err := manage.DeleteArtifact(path); err != nil {
		return err
	}
	printSuccess("deleted %s", accent(path))
	return nil
}

// confirmDeletion prompts before a delete: a whole config dir must have its
// name typed back; anything else is a yes/no.
func confirmDeletion(path string) (bool, error) {
	_, isConfigDir := scaffolding.HarnessForMarker(filepath.Base(path))

	if isConfigDir {
		typed := ""
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Delete the WHOLE config dir %s?", path)).
				Description(fmt.Sprintf("Type %q to confirm", filepath.Base(path))).
				Value(&typed),
		)).WithTheme(brandTheme())
		if err := form.Run(); err != nil {
			return false, err
		}
		return typed == filepath.Base(path), nil
	}

	confirmed := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Delete %s?", path)).
			Value(&confirmed),
	)).WithTheme(brandTheme())
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}
