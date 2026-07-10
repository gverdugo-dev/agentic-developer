// Package discovery scans a folder tree for AI harness config directories
// (.claude, .codex, .opencode) and summarizes what lives inside each one.
//
// The walk never enters ignored territory: an embedded default ignore list
// (dependency folders, caches, build output) applies everywhere, and every
// .gitignore found along the tree applies to its own subtree, so the scan
// respects the same boundaries git does. Symlinks are not followed and
// unreadable directories are skipped, not fatal.
package discovery

import (
	"agentic-developer/internal/scaffolding"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ConfigDir is one discovered harness config directory and its summary.
type ConfigDir struct {
	// Path is the absolute path of the config dir itself.
	Path string
	// Harness is the tool the dir belongs to, resolved from its name.
	Harness scaffolding.AIHarness
	// Skills are the skills under <Path>/skills, sorted by name.
	Skills []Skill
	// Plugins and Marketplaces come from the harness's own registry when the
	// config dir has one (Claude), the same data its /plugins screen shows;
	// otherwise they fall back to the folder layout.
	Plugins      []Plugin
	Marketplaces []Marketplace
}

// Skill is one skill folder, with the metadata parsed from its SKILL.md
// frontmatter.
type Skill struct {
	Name        string
	Path        string
	Description string
}

// Plugin is one plugin available in a config dir. Marketplace and Enabled
// are only populated when the plugin comes from a harness registry; Version
// and Description come from the registry or the plugin's own manifest.
type Plugin struct {
	Name        string
	Marketplace string
	Version     string
	Enabled     bool
	Path        string
	Description string
}

// Marketplace is one plugin marketplace registered in a config dir. Source
// is the human-readable origin ("github owner/repo", "directory /path"),
// empty for folder-based marketplaces. PluginNames is the catalog its
// manifest offers.
type Marketplace struct {
	Name        string
	Source      string
	Path        string
	PluginNames []string
}

// Scan walks the tree under root and returns every harness config dir found,
// sorted by path. root must be an existing directory. The config dirs
// sitting directly in the user's home (~/.claude, ~/.codex, ~/.opencode) are
// always included, at the head of the results, whatever the root: the
// user-level config applies to every project, so every scan surfaces it.
func Scan(root string) ([]ConfigDir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", abs)
	}

	var found []ConfigDir
	walkDir(abs, abs, nil, &found)

	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return prependUserConfigs(found), nil
}

// prependUserConfigs puts the harness config dirs living directly in the
// user's home at the head of found, skipping any the walk already collected
// (a scan rooted at the home itself finds them on its own).
func prependUserConfigs(found []ConfigDir) []ConfigDir {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return found
	}

	seen := make(map[string]bool, len(found))
	for _, dir := range found {
		seen[dir.Path] = true
	}

	var user []ConfigDir
	for _, harness := range scaffolding.HarnessesInOrder() {
		path := filepath.Join(home, scaffolding.MarkerFor(harness))
		if seen[path] {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		user = append(user, collectConfigDir(path, harness))
	}

	return append(user, found...)
}

// collectConfigDir summarizes one config dir. Claude dirs prefer their own
// plugin registry (what /plugins shows); every other case lists the artifact
// folders the scaffolder lays out.
func collectConfigDir(path string, harness scaffolding.AIHarness) ConfigDir {
	dir := ConfigDir{
		Path:    path,
		Harness: harness,
		Skills:  collectSkills(path),
	}

	if harness == scaffolding.Claude {
		if plugins, ok := claudePlugins(path); ok {
			dir.Plugins = plugins
		} else {
			dir.Plugins = folderPlugins(path)
		}
		if marketplaces, ok := claudeMarketplaces(path); ok {
			dir.Marketplaces = marketplaces
		} else {
			dir.Marketplaces = folderMarketplaces(path)
		}
		return dir
	}

	dir.Plugins = folderPlugins(path)
	dir.Marketplaces = folderMarketplaces(path)
	return dir
}

// collectSkills lists the skills of a config dir, each with the metadata
// parsed from its SKILL.md.
func collectSkills(configDir string) []Skill {
	var skills []Skill
	for _, name := range artifactDirs(configDir, "skills") {
		path := filepath.Join(configDir, "skills", name)
		skills = append(skills, Skill{
			Name:        name,
			Path:        path,
			Description: skillDescription(path),
		})
	}
	return skills
}

// folderPlugins lists plugins by folder layout, for config dirs without a
// registry, reading each plugin's own manifest for its metadata.
func folderPlugins(configDir string) []Plugin {
	var plugins []Plugin
	for _, name := range artifactDirs(configDir, "plugins") {
		path := filepath.Join(configDir, "plugins", name)
		man := readPluginManifest(path)
		plugins = append(plugins, Plugin{
			Name:        name,
			Version:     man.Version,
			Path:        path,
			Description: man.Description,
		})
	}
	return plugins
}

// folderMarketplaces lists marketplaces by folder layout, for config dirs
// without a registry, reading each catalog for the plugins it offers.
func folderMarketplaces(configDir string) []Marketplace {
	var marketplaces []Marketplace
	for _, name := range artifactDirs(configDir, "marketplaces") {
		path := filepath.Join(configDir, "marketplaces", name)
		marketplaces = append(marketplaces, Marketplace{
			Name:        name,
			Path:        path,
			PluginNames: marketplacePluginNames(path),
		})
	}
	return marketplaces
}

// walkDir recurses through dir collecting config dirs into found. ignores is
// the stack of .gitignore scopes accumulated from root down to dir; the
// matcher of each scope only ever sees paths relative to its own base, which
// is what gives nested .gitignore files their git semantics.
func walkDir(root, dir string, ignores []scopedIgnore, found *[]ConfigDir) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// An unreadable dir (permissions, races) is skipped, not fatal.
		return
	}

	if matcher := loadGitignore(dir); matcher != nil {
		ignores = append(ignores, scopedIgnore{base: dir, matcher: matcher})
	}

	for _, entry := range entries {
		// entry.IsDir is false for symlinks, so they are never followed.
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		path := filepath.Join(dir, name)

		// A config dir is a leaf of the scan: it is collected and never
		// descended into, so nothing inside one counts twice.
		if harness, ok := scaffolding.HarnessForMarker(name); ok {
			*found = append(*found, collectConfigDir(path, harness))
			continue
		}

		if isIgnored(root, path, ignores) {
			continue
		}

		walkDir(root, path, ignores, found)
	}
}

// artifactDirs lists the artifact folders under configDir/<container>,
// sorted. An artifact is any subdirectory there, matching how every
// supported harness lays them out. No container, or an unreadable one,
// yields nil.
func artifactDirs(configDir, container string) []string {
	entries, err := os.ReadDir(filepath.Join(configDir, container))
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}
