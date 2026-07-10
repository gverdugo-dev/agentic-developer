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
// ReadClaudeRegistry exposes the raw registry so other packages (the doctor
// checks) can cross-reference it; claudePlugins and claudeMarketplaces cook
// it into the discovery listing. A registry file that is missing or
// unparsable makes the listing fall back to the folder layout, so a
// scaffolded-but-unregistered config dir still reports something.

// PluginInstall is one install record of a registry plugin: the version and
// where in the plugin cache it is materialized.
type PluginInstall struct {
	Version     string `json:"version"`
	InstallPath string `json:"installPath"`
}

// RegisteredMarketplace is one known_marketplaces.json entry: the kind of
// source it was added from ("github", "git", "directory"), the source
// coordinates (exactly one of Repo, URL or Path is set, matching the kind),
// and where the marketplace is materialized on disk.
type RegisteredMarketplace struct {
	Kind            string
	Repo            string
	URL             string
	Path            string
	InstallLocation string
}

// SourceLabel is the human-readable origin of the marketplace: the kind
// followed by its coordinates ("github owner/repo", "directory /path").
func (m RegisteredMarketplace) SourceLabel() string {
	label := m.Kind
	switch {
	case m.Repo != "":
		label += " " + m.Repo
	case m.URL != "":
		label += " " + m.URL
	case m.Path != "":
		label += " " + m.Path
	}
	return label
}

// ClaudeRegistry is the raw plugin registry of one Claude config dir. Each
// map is nil when its file is missing or unparsable; HasInstalled and
// HasMarketplaces distinguish "readable registry" from "no registry", which
// is what decides the fallback to the folder listing.
type ClaudeRegistry struct {
	// InstalledPlugins maps "name@marketplace" to its install records; the
	// last record is the most recent install.
	InstalledPlugins map[string][]PluginInstall
	// KnownMarketplaces maps each registered marketplace name to its source.
	KnownMarketplaces map[string]RegisteredMarketplace
	// EnabledPlugins maps "name@marketplace" to its enabled state, from the
	// config dir's settings.json.
	EnabledPlugins map[string]bool
	// HasInstalled and HasMarketplaces report whether the corresponding
	// registry file was present and parsable.
	HasInstalled    bool
	HasMarketplaces bool
}

// ReadClaudeRegistry reads the raw plugin registry of a Claude config dir.
// Every part is best-effort: a missing or unparsable file leaves its field
// nil and its Has flag false, never an error.
func ReadClaudeRegistry(configDir string) ClaudeRegistry {
	reg := ClaudeRegistry{EnabledPlugins: enabledPlugins(configDir)}

	var installed struct {
		Plugins map[string][]PluginInstall `json:"plugins"`
	}
	if raw, err := os.ReadFile(filepath.Join(configDir, "plugins", "installed_plugins.json")); err == nil {
		if err := json.Unmarshal(raw, &installed); err == nil {
			reg.InstalledPlugins = installed.Plugins
			reg.HasInstalled = true
		}
	}

	// knownMarketplacesFile mirrors plugins/known_marketplaces.json.
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

// claudePlugins reads the installed plugins of a Claude config dir and their
// enabled state, or reports false when the dir has no readable registry.
func claudePlugins(configDir string) ([]Plugin, bool) {
	reg := ReadClaudeRegistry(configDir)
	if !reg.HasInstalled {
		return nil, false
	}

	plugins := make([]Plugin, 0, len(reg.InstalledPlugins))
	for key, installs := range reg.InstalledPlugins {
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
			Enabled:     reg.EnabledPlugins[key],
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

// claudeMarketplaces reads the registered marketplaces of a Claude config
// dir, or reports false when the dir has no readable registry.
func claudeMarketplaces(configDir string) ([]Marketplace, bool) {
	reg := ReadClaudeRegistry(configDir)
	if !reg.HasMarketplaces {
		return nil, false
	}

	marketplaces := make([]Marketplace, 0, len(reg.KnownMarketplaces))
	for name, entry := range reg.KnownMarketplaces {
		marketplaces = append(marketplaces, Marketplace{
			Name:        name,
			Source:      entry.SourceLabel(),
			Path:        entry.InstallLocation,
			PluginNames: marketplacePluginNames(entry.InstallLocation),
		})
	}

	sort.Slice(marketplaces, func(i, j int) bool {
		return marketplaces[i].Name < marketplaces[j].Name
	})
	return marketplaces, true
}
