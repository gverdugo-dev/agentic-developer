package doctor

import (
	"agentic-developer/internal/discovery"
	"fmt"
	"os"
	"path/filepath"
)

// checkRegistry cross-references the plugin registry of a Claude config dir
// against what is actually on disk: plugins enabled but not installed,
// installs whose cache folder is gone, cache folders no install claims, and
// directory-source marketplaces whose folder was deleted. A config dir
// without a registry (a scaffolded project .claude) yields nothing.
func checkRegistry(dir discovery.ConfigDir) []Finding {
	reg := discovery.ReadClaudeRegistry(dir.Path)
	var findings []Finding

	// A plugin enabled in settings.json but absent from the registry is a
	// ghost: the harness tries to load something that is not there.
	for key, enabled := range reg.EnabledPlugins {
		if !enabled {
			continue
		}
		if _, installed := reg.InstalledPlugins[key]; !installed {
			findings = append(findings, Finding{
				Severity: Error,
				Check:    "plugin-enabled-not-installed",
				Path:     filepath.Join(dir.Path, "settings.json"),
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
				findings = append(findings, Finding{
					Severity: Error,
					Check:    "plugin-cache-missing",
					Path:     install.InstallPath,
					Message:  fmt.Sprintf("plugin %q %s is registered but its install path is gone", key, install.Version),
					FixHint:  fmt.Sprintf("reinstall it (claude plugin uninstall %s && claude plugin install %s)", key, key),
				})
			}
		}

		// A cache folder no registry entry claims is dead weight.
		for _, entry := range cacheEntries(dir.Path) {
			if _, installed := reg.InstalledPlugins[entry.key]; !installed {
				findings = append(findings, Finding{
					Severity: Warning,
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
		findings = append(findings, Finding{
			Severity: Error,
			Check:    "marketplace-dir-missing",
			Path:     mkt.Path,
			Message:  fmt.Sprintf("marketplace %q points at directory %s, which no longer exists", name, mkt.Path),
			FixHint:  fmt.Sprintf("restore the directory, or remove the marketplace (claude plugin marketplace remove %s)", name),
		})
	}

	return findings
}

// cacheEntry is one plugins/cache/<marketplace>/<plugin> folder, keyed the
// way the registry keys installs.
type cacheEntry struct {
	key  string // plugin@marketplace
	path string
}

// cacheEntries lists the plugin folders of the config dir's plugin cache,
// which is laid out as plugins/cache/<marketplace>/<plugin>/<version>. A
// missing or unreadable cache yields nothing.
func cacheEntries(configDir string) []cacheEntry {
	cacheDir := filepath.Join(configDir, "plugins", "cache")
	marketplaces, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil
	}

	var entries []cacheEntry
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
			entries = append(entries, cacheEntry{
				key:  plugin.Name() + "@" + mkt.Name(),
				path: filepath.Join(cacheDir, mkt.Name(), plugin.Name()),
			})
		}
	}
	return entries
}
