package harness

import (
	"agentic-developer/internal/scaffolding"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// claudeAdapter is the deep harness: Claude Code keeps its own plugin
// registry inside the config dir (the same data its /plugins screen shows),
// and the claude CLI owns every mutation of it. Three files participate:
//
//	plugins/installed_plugins.json    what is installed, per "name@marketplace"
//	plugins/known_marketplaces.json   the registered marketplaces and sources
//	settings.json (enabledPlugins)    which installed plugins are enabled
type claudeAdapter struct{}

func (claudeAdapter) ID() scaffolding.AIHarness { return scaffolding.Claude }
func (a claudeAdapter) Name() string            { return scaffolding.AIHarnesses[a.ID()] }
func (a claudeAdapter) Marker() string          { return scaffolding.MarkerFor(a.ID()) }

// Containers: Claude holds all three scaffolded artifact kinds.
func (claudeAdapter) Containers() []Container {
	return []Container{
		{Kind: KindSkill, Dir: "skills"},
		{Kind: KindPlugin, Dir: "plugins"},
		{Kind: KindMarketplace, Dir: "marketplaces"},
	}
}

// InstructionFiles: Claude loads CLAUDE.md as instructions, user-level
// inside the config dir (~/.claude/CLAUDE.md) and project-level next to it.
func (claudeAdapter) InstructionFiles(configDir string) []string {
	return existingFiles(configDir, "CLAUDE.md")
}

// Registry reads the raw plugin registry of a Claude config dir. Every part
// is best-effort: a missing or unparsable file leaves its field nil and its
// Has flag false, never an error.
func (claudeAdapter) Registry(configDir string) Registry {
	reg := Registry{EnabledPlugins: claudeEnabledPlugins(configDir)}

	var installed struct {
		Plugins map[string][]PluginInstall `json:"plugins"`
	}
	if raw, err := os.ReadFile(filepath.Join(configDir, "plugins", "installed_plugins.json")); err == nil {
		if err := json.Unmarshal(raw, &installed); err == nil {
			reg.InstalledPlugins = installed.Plugins
			reg.HasInstalled = true
		}
	}

	// known mirrors plugins/known_marketplaces.json.
	var known map[string]struct {
		Source struct {
			Source string `json:"source"`
			Repo   string `json:"repo"`
			URL    string `json:"url"`
			Path   string `json:"path"`
		} `json:"source"`
		InstallLocation string `json:"installLocation"`
	}
	if raw, err := os.ReadFile(filepath.Join(configDir, "plugins", "known_marketplaces.json")); err == nil {
		if err := json.Unmarshal(raw, &known); err == nil {
			reg.KnownMarketplaces = make(map[string]RegisteredMarketplace, len(known))
			for name, entry := range known {
				reg.KnownMarketplaces[name] = RegisteredMarketplace{
					Kind:            entry.Source.Source,
					Repo:            entry.Source.Repo,
					URL:             entry.Source.URL,
					Path:            entry.Source.Path,
					InstallLocation: entry.InstallLocation,
				}
			}
			reg.HasMarketplaces = true
		}
	}

	return reg
}

// claudeEnabledPlugins reads the enabledPlugins map from the config dir's
// settings.json; a missing or unparsable file just means nothing is marked
// enabled.
func claudeEnabledPlugins(configDir string) map[string]bool {
	raw, err := os.ReadFile(filepath.Join(configDir, "settings.json"))
	if err != nil {
		return nil
	}
	var settings struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil
	}
	return settings.EnabledPlugins
}

// PluginMeta parses the plugin's .claude-plugin/plugin.json, returning the
// zero value when there is none.
func (claudeAdapter) PluginMeta(dir string) PluginMeta {
	var meta PluginMeta
	raw, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
	if err != nil {
		return meta
	}
	var man struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &man); err != nil {
		return meta
	}
	return PluginMeta{Name: man.Name, Version: man.Version, Description: man.Description}
}

// MarketplaceMeta parses the marketplace's .claude-plugin/marketplace.json:
// its name and the catalog of plugins it offers.
func (claudeAdapter) MarketplaceMeta(dir string) MarketplaceMeta {
	raw, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "marketplace.json"))
	if err != nil {
		return MarketplaceMeta{}
	}
	var man struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &man); err != nil {
		return MarketplaceMeta{}
	}

	meta := MarketplaceMeta{Name: man.Name}
	for _, p := range man.Plugins {
		if p.Name != "" {
			meta.PluginNames = append(meta.PluginNames, p.Name)
		}
	}
	return meta
}

// ValidatePlugin checks the plugin's .claude-plugin/plugin.json manifest.
func (claudeAdapter) ValidatePlugin(name, dir string) []Issue {
	return claudeManifestIssues("plugin", name, dir, "plugin.json")
}

// ValidateMarketplace checks the marketplace's .claude-plugin/marketplace.json.
func (claudeAdapter) ValidateMarketplace(name, dir string) []Issue {
	return claudeManifestIssues("marketplace", name, dir, "marketplace.json")
}

// claudeManifestIssues validates one .claude-plugin manifest: it must exist,
// parse as JSON, and carry a name. Folders that do not exist are not this
// check's business: the registry checks flag those.
func claudeManifestIssues(kind, name, dir, manifestName string) []Issue {
	if dir == "" || !isDir(dir) {
		return nil
	}

	file := filepath.Join(dir, ".claude-plugin", manifestName)
	raw, err := os.ReadFile(file)
	if err != nil {
		return []Issue{{
			Severity: IssueError,
			Check:    kind + "-manifest-missing",
			Path:     dir,
			Message:  fmt.Sprintf("%s %q has no .claude-plugin/%s, so the harness cannot load it", kind, name, manifestName),
			FixHint:  fmt.Sprintf("create .claude-plugin/%s with at least a name field", manifestName),
		}}
	}

	var manifest struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return []Issue{{
			Severity: IssueError,
			Check:    kind + "-manifest-invalid",
			Path:     file,
			Message:  fmt.Sprintf("%s of %s %q is not valid JSON: %v", manifestName, kind, name, err),
			FixHint:  "fix the JSON syntax",
		}}
	}

	if strings.TrimSpace(manifest.Name) == "" {
		return []Issue{{
			Severity: IssueWarning,
			Check:    kind + "-name-missing",
			Path:     file,
			Message:  fmt.Sprintf("%s of %s %q has no name field", manifestName, kind, name),
			FixHint:  "add a name matching the folder, in kebab-case",
		}}
	}

	return nil
}

// ValidateRegistry cross-references the plugin registry of a Claude config
// dir against what is actually on disk: plugins enabled but not installed,
// installs whose cache folder is gone, cache folders no install claims, and
// directory-source marketplaces whose folder was deleted. A config dir
// without a registry (a scaffolded project .claude) yields nothing.
func (a claudeAdapter) ValidateRegistry(configDir string) []Issue {
	reg := a.Registry(configDir)
	var issues []Issue

	// A plugin enabled in settings.json but absent from the registry is a
	// ghost: the harness tries to load something that is not there.
	for key, enabled := range reg.EnabledPlugins {
		if !enabled {
			continue
		}
		if _, installed := reg.InstalledPlugins[key]; !installed {
			issues = append(issues, Issue{
				Severity: IssueError,
				Check:    "plugin-enabled-not-installed",
				Path:     filepath.Join(configDir, "settings.json"),
				Message:  fmt.Sprintf("plugin %q is enabled in settings.json but not installed", key),
				FixHint:  fmt.Sprintf("install it (claude plugin install %s) or remove it from enabledPlugins", key),
			})
		}
	}

	if reg.HasInstalled {
		// A registry install whose cache folder is gone cannot load.
		for key, installs := range reg.InstalledPlugins {
			for _, install := range installs {
				if install.InstallPath == "" || isDir(install.InstallPath) {
					continue
				}
				issues = append(issues, Issue{
					Severity: IssueError,
					Check:    "plugin-cache-missing",
					Path:     install.InstallPath,
					Message:  fmt.Sprintf("plugin %q %s is registered but its install path is gone", key, install.Version),
					FixHint:  fmt.Sprintf("reinstall it (claude plugin uninstall %s && claude plugin install %s)", key, key),
				})
			}
		}

		// A cache folder no registry entry claims is dead weight.
		for _, entry := range claudeCacheEntries(configDir) {
			if _, installed := reg.InstalledPlugins[entry.key]; !installed {
				issues = append(issues, Issue{
					Severity: IssueWarning,
					Check:    "cache-orphan",
					Path:     entry.path,
					Message:  fmt.Sprintf("cached plugin %q has no registry entry", entry.key),
					FixHint:  "delete the cache folder, or reinstall the plugin to re-register it",
				})
			}
		}
	}

	// A directory-source marketplace reads straight from its path; when the
	// path is deleted every plugin installed from it rots.
	for name, mkt := range reg.KnownMarketplaces {
		if mkt.Kind != "directory" || isDir(mkt.Path) {
			continue
		}
		issues = append(issues, Issue{
			Severity: IssueError,
			Check:    "marketplace-dir-missing",
			Path:     mkt.Path,
			Message:  fmt.Sprintf("marketplace %q points at directory %s, which no longer exists", name, mkt.Path),
			FixHint:  fmt.Sprintf("restore the directory, or remove the marketplace (claude plugin marketplace remove %s)", name),
		})
	}

	return issues
}

// claudeCacheEntry is one plugins/cache/<marketplace>/<plugin> folder, keyed
// the way the registry keys installs.
type claudeCacheEntry struct {
	key  string // plugin@marketplace
	path string
}

// claudeCacheEntries lists the plugin folders of the config dir's plugin
// cache, which is laid out as plugins/cache/<marketplace>/<plugin>/<version>.
// A missing or unreadable cache yields nothing.
func claudeCacheEntries(configDir string) []claudeCacheEntry {
	cacheDir := filepath.Join(configDir, "plugins", "cache")
	marketplaces, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil
	}

	var entries []claudeCacheEntry
	for _, mkt := range marketplaces {
		if !mkt.IsDir() {
			continue
		}
		plugins, err := os.ReadDir(filepath.Join(cacheDir, mkt.Name()))
		if err != nil {
			continue
		}
		for _, plugin := range plugins {
			if !plugin.IsDir() {
				continue
			}
			entries = append(entries, claudeCacheEntry{
				key:  plugin.Name() + "@" + mkt.Name(),
				path: filepath.Join(cacheDir, mkt.Name(), plugin.Name()),
			})
		}
	}
	return entries
}

// Operations: the claude CLI owns the registry, so every mutation is
// delegated to it.
func (claudeAdapter) Operations() Operations { return claudeOperations{} }

// ClaudeExec invokes the claude CLI and returns its trimmed combined output.
// It is a variable so tests (and any caller that must not touch the real
// harness) can stub it. Registry-touching operations always go through
// claude itself: it owns installed_plugins.json, the plugin cache and
// settings, and writing those by hand risks corrupting its state.
var ClaudeExec = func(args ...string) (string, error) {
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

// claudeOperations implements Operations over the claude CLI.
type claudeOperations struct{}

// InstallPlugin installs a plugin ("name@marketplace") from a registered
// marketplace, which downloads it into the cache and records it in the
// registry.
func (claudeOperations) InstallPlugin(key string) (string, error) {
	return ClaudeExec("plugin", "install", key)
}

// UninstallPlugin uninstalls a registry plugin ("name@marketplace"), which
// also cleans its cache and registry entries.
func (claudeOperations) UninstallPlugin(key string) (string, error) {
	return ClaudeExec("plugin", "uninstall", key)
}

// SetPluginEnabled enables or disables an installed plugin
// ("name@marketplace").
func (claudeOperations) SetPluginEnabled(key string, enabled bool) (string, error) {
	action := "disable"
	if enabled {
		action = "enable"
	}
	return ClaudeExec("plugin", action, key)
}

// AddMarketplace registers a plugin marketplace. source is whatever claude
// accepts: a GitHub "owner/repo", a git URL, or a local path.
func (claudeOperations) AddMarketplace(source string) (string, error) {
	return ClaudeExec("plugin", "marketplace", "add", source)
}

// RemoveMarketplace unregisters a marketplace by name.
func (claudeOperations) RemoveMarketplace(name string) (string, error) {
	return ClaudeExec("plugin", "marketplace", "remove", name)
}
