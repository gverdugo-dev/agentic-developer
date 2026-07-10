package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// This file reads Claude Code's own plugin registry inside a config dir: the
// same data its /plugins screen shows. Three files participate:
//
//   plugins/installed_plugins.json    what is installed, per "name@marketplace"
//   plugins/known_marketplaces.json   the registered marketplaces and sources
//   settings.json (enabledPlugins)    which installed plugins are enabled
//
// A registry file that is missing or unparsable makes the caller fall back
// to the folder-based listing, so a scaffolded-but-unregistered config dir
// still reports something.

// installedPluginsFile mirrors plugins/installed_plugins.json (version 2):
// every "name@marketplace" key maps to its list of installs.
type installedPluginsFile struct {
	Plugins map[string][]struct {
		Version     string `json:"version"`
		InstallPath string `json:"installPath"`
	} `json:"plugins"`
}

// claudePlugins reads the installed plugins of a Claude config dir and their
// enabled state, or reports false when the dir has no readable registry.
func claudePlugins(configDir string) ([]Plugin, bool) {
	raw, err := os.ReadFile(filepath.Join(configDir, "plugins", "installed_plugins.json"))
	if err != nil {
		return nil, false
	}
	var file installedPluginsFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, false
	}

	enabled := enabledPlugins(configDir)

	plugins := make([]Plugin, 0, len(file.Plugins))
	for key, installs := range file.Plugins {
		name, marketplace, _ := strings.Cut(key, "@")
		version, installPath := "", ""
		if len(installs) > 0 {
			// The last entry is the most recent install of the plugin.
			version = installs[len(installs)-1].Version
			installPath = installs[len(installs)-1].InstallPath
		}
		plugins = append(plugins, Plugin{
			Name:        name,
			Marketplace: marketplace,
			Version:     version,
			Enabled:     enabled[key],
			Path:        installPath,
			Description: readPluginManifest(installPath).Description,
		})
	}

	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Name != plugins[j].Name {
			return plugins[i].Name < plugins[j].Name
		}
		return plugins[i].Marketplace < plugins[j].Marketplace
	})
	return plugins, true
}

// enabledPlugins reads the enabledPlugins map from the config dir's
// settings.json; a missing or unparsable file just means nothing is marked
// enabled.
func enabledPlugins(configDir string) map[string]bool {
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

// knownMarketplacesFile mirrors plugins/known_marketplaces.json: marketplace
// name to its source (exactly one of repo, url or path is set, depending on
// the source kind).
type knownMarketplacesFile map[string]struct {
	Source struct {
		Source string `json:"source"`
		Repo   string `json:"repo"`
		URL    string `json:"url"`
		Path   string `json:"path"`
	} `json:"source"`
	InstallLocation string `json:"installLocation"`
}

// claudeMarketplaces reads the registered marketplaces of a Claude config
// dir, or reports false when the dir has no readable registry.
func claudeMarketplaces(configDir string) ([]Marketplace, bool) {
	raw, err := os.ReadFile(filepath.Join(configDir, "plugins", "known_marketplaces.json"))
	if err != nil {
		return nil, false
	}
	var file knownMarketplacesFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, false
	}

	marketplaces := make([]Marketplace, 0, len(file))
	for name, entry := range file {
		source := entry.Source.Source
		switch {
		case entry.Source.Repo != "":
			source += " " + entry.Source.Repo
		case entry.Source.URL != "":
			source += " " + entry.Source.URL
		case entry.Source.Path != "":
			source += " " + entry.Source.Path
		}
		marketplaces = append(marketplaces, Marketplace{
			Name:        name,
			Source:      source,
			Path:        entry.InstallLocation,
			PluginNames: marketplacePluginNames(entry.InstallLocation),
		})
	}

	sort.Slice(marketplaces, func(i, j int) bool {
		return marketplaces[i].Name < marketplaces[j].Name
	})
	return marketplaces, true
}
