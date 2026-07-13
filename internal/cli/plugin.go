package cli

import (
	"agentic-developer/internal/manage"
	"flag"
	"fmt"
)

// pluginCmd implements the plugin command: install, enable, disable or
// uninstall a plugin, delegated to the claude CLI (which owns the registry
// and the cache). The CLI face of the TUI's "i" (install), "t" (toggle) and
// "d" on registry plugins.
type pluginCmd struct{}

// Name returns the command's CLI word.
func (pluginCmd) Name() string { return "plugin" }

// Synopsis returns the one-line help for the command.
func (pluginCmd) Synopsis() string {
	return "install, enable, disable or uninstall a plugin (via the claude CLI)"
}

// pluginActions maps each action word to its manage operation and the past
// tense the success line uses.
var pluginActions = map[string]struct {
	run  func(key string) (string, error)
	done string
}{
	"install":   {manage.InstallPlugin, "installed"},
	"enable":    {func(key string) (string, error) { return manage.SetPluginEnabled(key, true) }, "enabled"},
	"disable":   {func(key string) (string, error) { return manage.SetPluginEnabled(key, false) }, "disabled"},
	"uninstall": {manage.UninstallPlugin, "uninstalled"},
}

// Run parses the action and plugin key positionals plus the --json flag and
// delegates to the manage layer.
func (c pluginCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev plugin", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the result as JSON on stdout")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev plugin <install|enable|disable|uninstall> <name@marketplace> [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) != 2 {
		fs.Usage()
		return fmt.Errorf("provide the action and the plugin key")
	}
	action, key := positional[0], positional[1]

	op, ok := pluginActions[action]
	if !ok {
		return fmt.Errorf("unknown action %q: use install, enable, disable or uninstall", action)
	}
	output, err := op.run(key)
	if err != nil {
		return err
	}

	if *asJSON {
		return jsonOut(struct {
			Action string `json:"action"`
			Plugin string `json:"plugin"`
			Output string `json:"output,omitempty"`
		}{Action: action, Plugin: key, Output: output})
	}

	if output != "" {
		printInfo("%s", muted(output))
	}
	printSuccess("%s %s", op.done, accent(key))
	return nil
}
