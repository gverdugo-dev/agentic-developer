package cli

import (
	"agentic-developer/internal/manage"
	"flag"
	"fmt"
)

// marketplaceCmd implements the marketplace command: register or remove a
// plugin marketplace, delegated to the claude CLI. The CLI face of the TUI's
// "a" (add) and "d" on registry marketplaces.
type marketplaceCmd struct{}

// Name returns the command's CLI word.
func (marketplaceCmd) Name() string { return "marketplace" }

// Synopsis returns the one-line help for the command.
func (marketplaceCmd) Synopsis() string {
	return "add or remove a plugin marketplace (via the claude CLI)"
}

// Run parses the action and its argument plus the --json flag and delegates
// to the manage layer.
func (c marketplaceCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev marketplace", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the result as JSON on stdout")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev marketplace <add <source>|remove <name>> [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\nadd takes a source: a GitHub owner/repo, a git URL, or a local path.")
		fmt.Fprintln(fs.Output(), "remove takes the registered marketplace name.")
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) != 2 {
		fs.Usage()
		return fmt.Errorf("provide the action (add or remove) and its argument")
	}
	action, arg := positional[0], positional[1]

	var output string
	var done string
	switch action {
	case "add":
		output, err = manage.AddMarketplace(arg)
		done = "added marketplace"
	case "remove":
		output, err = manage.RemoveMarketplace(arg)
		done = "removed marketplace"
	default:
		return fmt.Errorf("unknown action %q: use add or remove", action)
	}
	if err != nil {
		return err
	}

	if *asJSON {
		return jsonOut(struct {
			Action      string `json:"action"`
			Marketplace string `json:"marketplace"`
			Output      string `json:"output,omitempty"`
		}{Action: action, Marketplace: arg, Output: output})
	}

	if output != "" {
		printInfo("%s", muted(output))
	}
	printSuccess("%s %s", done, accent(arg))
	return nil
}
