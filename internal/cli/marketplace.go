package cli

import (
	"agentic-developer/internal/manage"
	"fmt"
)

// marketplaceCmd implements the marketplace command: for now removing a
// registered marketplace, delegated to the claude CLI. Adding one comes with
// the install phase.
type marketplaceCmd struct{}

// Name returns the command's CLI word.
func (marketplaceCmd) Name() string { return "marketplace" }

// Synopsis returns the one-line help for the command.
func (marketplaceCmd) Synopsis() string {
	return "remove a registered plugin marketplace (via the claude CLI)"
}

// Run parses the action and marketplace name positionals and delegates.
func (c marketplaceCmd) Run(args []string) error {
	if len(args) != 2 || args[0] != "remove" {
		printInfo("Usage: adev marketplace remove <name>")
		return fmt.Errorf("provide the action (remove) and the marketplace name")
	}

	output, err := manage.RemoveMarketplace(args[1])
	if err != nil {
		return err
	}

	if output != "" {
		printInfo("%s", muted(output))
	}
	printSuccess("removed marketplace %s", accent(args[1]))
	return nil
}
