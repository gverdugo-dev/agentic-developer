package manage

import (
	"agentic-developer/internal/harness"
	"agentic-developer/internal/scaffolding"
)

// The plugin and marketplace operations forward to the Claude adapter's
// executor (internal/harness), which owns the claude CLI delegation: claude
// owns installed_plugins.json, the plugin cache and settings, and writing
// those by hand risks corrupting its state. The named helpers stay here so
// the TUI and the CLI commands share one mutation layer; tests stub
// harness.ClaudeExec to intercept the underlying CLI calls.

// claudeOps returns the Claude adapter's operation executor.
func claudeOps() harness.Operations {
	ad, _ := harness.ForID(scaffolding.Claude)
	return ad.Operations()
}

// SetPluginEnabled enables or disables an installed plugin
// ("name@marketplace") via the claude CLI.
func SetPluginEnabled(key string, enabled bool) (string, error) {
	return claudeOps().SetPluginEnabled(key, enabled)
}

// InstallPlugin installs a plugin ("name@marketplace") from a registered
// marketplace via the claude CLI, which downloads it into the cache and
// records it in the registry.
func InstallPlugin(key string) (string, error) {
	return claudeOps().InstallPlugin(key)
}

// UninstallPlugin uninstalls a registry plugin ("name@marketplace") via the
// claude CLI, which also cleans its cache and registry entries.
func UninstallPlugin(key string) (string, error) {
	return claudeOps().UninstallPlugin(key)
}

// AddMarketplace registers a plugin marketplace via the claude CLI. source is
// whatever claude accepts: a GitHub "owner/repo", a git URL, or a local path.
func AddMarketplace(source string) (string, error) {
	return claudeOps().AddMarketplace(source)
}

// RemoveMarketplace unregisters a marketplace by name via the claude CLI.
func RemoveMarketplace(name string) (string, error) {
	return claudeOps().RemoveMarketplace(name)
}
