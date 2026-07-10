package manage

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Exec invokes the claude CLI and returns its trimmed combined output. It is
// a variable so tests (and any caller that must not touch the real harness)
// can stub it. Registry-touching operations always go through claude itself:
// it owns installed_plugins.json, the plugin cache and settings, and writing
// those by hand risks corrupting its state.
var Exec = func(args ...string) (string, error) {
	out, err := exec.Command("claude", args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("claude CLI not found in PATH")
		}
		if text != "" {
			return "", fmt.Errorf("claude %s: %s", strings.Join(args, " "), text)
		}
		return "", fmt.Errorf("claude %s: %w", strings.Join(args, " "), err)
	}
	return text, nil
}

// SetPluginEnabled enables or disables an installed plugin
// ("name@marketplace") via the claude CLI.
func SetPluginEnabled(key string, enabled bool) (string, error) {
	action := "disable"
	if enabled {
		action = "enable"
	}
	return Exec("plugin", action, key)
}

// InstallPlugin installs a plugin ("name@marketplace") from a registered
// marketplace via the claude CLI, which downloads it into the cache and
// records it in the registry.
func InstallPlugin(key string) (string, error) {
	return Exec("plugin", "install", key)
}

// UninstallPlugin uninstalls a registry plugin ("name@marketplace") via the
// claude CLI, which also cleans its cache and registry entries.
func UninstallPlugin(key string) (string, error) {
	return Exec("plugin", "uninstall", key)
}

// AddMarketplace registers a plugin marketplace via the claude CLI. source is
// whatever claude accepts: a GitHub "owner/repo", a git URL, or a local path.
func AddMarketplace(source string) (string, error) {
	return Exec("plugin", "marketplace", "add", source)
}

// RemoveMarketplace unregisters a marketplace by name via the claude CLI.
func RemoveMarketplace(name string) (string, error) {
	return Exec("plugin", "marketplace", "remove", name)
}
