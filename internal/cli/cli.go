// Package cli is the boundary layer between the raw process args and the typed
// domain in package scaffolding. It models every invocation as a Command: each
// command owns the parsing of its own flags and positional args and runs
// itself, and Run dispatches to the one named on the command line.
package cli

import (
	"agentic-developer/internal/scaffolding"
	"agentic-developer/internal/tui"
	"errors"
	"fmt"
	"os"
)

// Command is a single adev subcommand (new, delete, setup, ...). Each one parses
// the args that follow its name and executes itself, so adding a command is a
// matter of implementing this interface and registering it, with no change to
// the dispatcher.
type Command interface {
	// Name is the word typed on the command line to select the command.
	Name() string
	// Synopsis is the one-line description shown in the top-level help.
	Synopsis() string
	// Run executes the command. args are the process args that follow the
	// command name, so each command parses its own flags and positionals.
	Run(args []string) error
}

// commands is the registry of every available command, keyed by its name.
var commands = map[string]Command{
	"new":         scaffoldCmd{verb: scaffolding.New},
	"delete":      scaffoldCmd{verb: scaffolding.Delete},
	"rm":          rmCmd{},
	"scan":        scanCmd{},
	"list":        listCmd{},
	"doctor":      doctorCmd{},
	"clean":       cleanCmd{},
	"plugin":      pluginCmd{},
	"marketplace": marketplaceCmd{},
	"setup":       setupCmd{},
	"update":      updateCmd{},
	"version":     versionCmd{},
}

// order fixes the listing order in the help output; map iteration is random.
var order = []string{"new", "delete", "rm", "scan", "list", "doctor", "clean", "plugin", "marketplace", "setup", "update", "version"}

// Run is the single entry point of the CLI: it selects the command named by the
// first arg and hands it the rest. It returns an error so main() can decide the
// exit code in one place.
//
// With no command at all, a terminal caller gets the TUI (the lazygit model:
// the bare binary IS the interactive tool), while a non-terminal caller (a
// script or CI) keeps the usage error, so nothing ever blocks on a prompt.
func Run(argv []string) error {
	if len(argv) < 2 {
		if interactiveAvailable() {
			// The initial discovery root is the caller's working directory,
			// the same way lazygit opens on the repo you are standing in.
			root, err := os.Getwd()
			if err != nil {
				root = "."
			}
			return tui.Run(resolveVersion(), root)
		}
		printUsage()
		return errors.New("no command given")
	}

	name := argv[1]
	switch name {
	case "-h", "--help", "help":
		printUsage()
		return nil
	case "-v", "--version":
		name = "version"
	}

	cmd, ok := commands[name]
	if !ok {
		printUsage()
		return fmt.Errorf("unknown command %q", name)
	}

	return cmd.Run(argv[2:])
}

// printUsage writes the top-level help: the command list with each synopsis.
func printUsage() {
	fmt.Println(title("adev") + " scaffolds and removes AI coding-agent artifacts.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("\tadev <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	for _, name := range order {
		// Pad the styled name to the visible width, then color it, so the
		// alignment stays correct even when the style adds invisible escapes.
		fmt.Printf("\t%s %s\n", command(fmt.Sprintf("%-9s", name)), commands[name].Synopsis())
	}
	fmt.Println()
	fmt.Println(muted("Run 'adev <command> -h' for command-specific help."))
	fmt.Println(muted("Run 'adev' with no arguments on a terminal to open the TUI."))
}
