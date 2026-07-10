package cli

import (
	"agentic-developer/internal/manage"
	"fmt"
)

// pluginCmd implements the plugin command: enable, disable or uninstall an
// installed plugin, delegated to the claude CLI (which owns the registry and
// the cache). The CLI face of the TUI's "t" (toggle) and "d" on registry
// plugins.
type pluginCmd struct{}

// Name returns the command's CLI word.
func (pluginCmd) Name() string { return "plugin" }

// Synopsis returns the one-line help for the command.
func (pluginCmd) Synopsis() string {
	return "enable, disable or uninstall a plugin (via the claude CLI)"
}

// Run parses the action and plugin key positionals and delegates.
func (c pluginCmd) Run(args []string) error {
	if len(args) != 2 {
		printInfo("Usage: adev plugin <enable|disable|uninstall> <name@marketplace>")
		return fmt.Errorf("provide the action and the plugin key")
	}
	action, key := args[0], args[1]

	var output string
	var err error
	switch action {
	case "enable":
		output, err = manage.SetPluginEnabled(key, true)
	case "disable":
		output, err = manage.SetPluginEnabled(key, false)
	case "uninstall":
		output, err = manage.UninstallPlugin(key)
	default:
		return fmt.Errorf("unknown action %q: use enable, disable or uninstall", action)
	}
	if err != nil {
		return err
	}

	if output != "" {
		printInfo("%s", muted(output))
	}
	printSuccess("%s %s", action+"d", accent(key))
	return nil
}
