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
	"agentic-developer/internal/harness"
	"agentic-developer/internal/scaffolding"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfigDir is one discovered harness config directory and its summary.
// What gets collected is driven entirely by the dir's harness adapter (see
// internal/harness): only the artifact kinds the harness actually supports
// are listed.
type ConfigDir struct {
	// Path is the absolute path of the config dir itself.
	Path string
	// Harness is the tool the dir belongs to, resolved from its name.
	Harness scaffolding.AIHarness
	// Skills are the skills under the harness's skills container, sorted by
	// name.
	Skills []Skill
	// Plugins and Marketplaces come from the harness's own registry when the
	// config dir has one (Claude), the same data its /plugins screen shows;
	// otherwise they fall back to the folder layout of the harness's
	// containers. Harnesses without the concept report none.
	Plugins      []Plugin
	Marketplaces []Marketplace
	// Prompts are the prompt files of harnesses that load reusable prompts
	// from a folder (Codex), sorted by name.
	Prompts []Prompt
	// Instructions are the harness's instruction/config files found in or
	// next to the config dir (CLAUDE.md, AGENTS.md, opencode.json).
	Instructions []string
}

// Skill is one skill folder, with the metadata parsed from its SKILL.md
// frontmatter. Hash is the stable content hash of the skill dir ("" when it
// could not be hashed); fileHashes backs the drifted-file counts of
// duplicate groups.
type Skill struct {
	Name        string
	Path        string
	Description string
	Hash        string
	fileHashes  map[string]string
}

// Plugin is one plugin available in a config dir. Marketplace and Enabled
// are only populated when the plugin comes from a harness registry; Version
// and Description come from the registry or the plugin's own manifest. Hash
// is the stable content hash of the plugin dir ("" when it could not be
// hashed); fileHashes backs the drifted-file counts of duplicate groups.
type Plugin struct {
	Name        string
	Marketplace string
	Version     string
	Enabled     bool
	Path        string
	Description string
	Hash        string
	fileHashes  map[string]string
}

// Marketplace is one plugin marketplace registered in a config dir. Source
// is the human-readable origin ("github owner/repo", "directory /path"),
// empty for folder-based marketplaces. PluginNames is the catalog its
// manifest offers. Hash is the stable content hash of the marketplace dir
// ("" when it could not be hashed); fileHashes backs the drifted-file
// counts of duplicate groups.
type Marketplace struct {
	Name        string
	Source      string
	Path        string
	PluginNames []string
	Hash        string
	fileHashes  map[string]string
}

// Prompt is one reusable prompt file of a harness that loads them from a
// folder (Codex's prompts dir, where each markdown file becomes a custom
// command).
type Prompt struct {
	Name string
	Path string
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
	for _, ad := range harness.All() {
		path := filepath.Join(home, ad.Marker())
		if seen[path] {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		user = append(user, collectConfigDir(path, ad))
	}

	return append(user, found...)
}

// collectConfigDir summarizes one config dir, collecting exactly the
// containers its adapter declares. Harnesses with their own plugin registry
// (Claude) prefer it, the same data their own UI shows; without a readable
// registry the folder layout is listed instead, so a
// scaffolded-but-unregistered config dir still reports something.
func collectConfigDir(path string, ad harness.Adapter) ConfigDir {
	dir := ConfigDir{
		Path:         path,
		Harness:      ad.ID(),
		Instructions: ad.InstructionFiles(path),
	}

	reg := ad.Registry(path)
	for _, c := range ad.Containers() {
		switch c.Kind {
		case harness.KindSkill:
			dir.Skills = collectSkills(path, c.Dir)
		case harness.KindPrompt:
			dir.Prompts = collectPrompts(path, c.Dir)
		case harness.KindPlugin:
			if reg.HasInstalled {
				dir.Plugins = registryPlugins(ad, reg)
			} else {
				dir.Plugins = folderPlugins(ad, path, c.Dir)
			}
		case harness.KindMarketplace:
			if reg.HasMarketplaces {
				dir.Marketplaces = registryMarketplaces(ad, reg)
			} else {
				dir.Marketplaces = folderMarketplaces(ad, path, c.Dir)
			}
		}
	}
	return dir
}

// collectSkills lists the skills under configDir/<container>, each with the
// metadata parsed from its SKILL.md.
func collectSkills(configDir, container string) []Skill {
	var skills []Skill
	for _, name := range artifactDirs(configDir, container) {
		path := filepath.Join(configDir, container, name)
		hash, files := hashArtifactDir(path)
		skills = append(skills, Skill{
			Name:        name,
			Path:        path,
			Description: skillDescription(path),
			Hash:        hash,
			fileHashes:  files,
		})
	}
	return skills
}

// collectPrompts lists the prompt files under configDir/<container>: every
// markdown file there, sorted by name. No container, or an unreadable one,
// yields nil.
func collectPrompts(configDir, container string) []Prompt {
	entries, err := os.ReadDir(filepath.Join(configDir, container))
	if err != nil {
		return nil
	}

	var prompts []Prompt
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		prompts = append(prompts, Prompt{
			Name: strings.TrimSuffix(entry.Name(), ".md"),
			Path: filepath.Join(configDir, container, entry.Name()),
		})
	}
	sort.Slice(prompts, func(i, j int) bool { return prompts[i].Name < prompts[j].Name })
	return prompts
}

// folderPlugins lists plugins by folder layout, for config dirs without a
// registry, reading each plugin's own manifest (whatever file that is for
// the harness) for its metadata.
func folderPlugins(ad harness.Adapter, configDir, container string) []Plugin {
	var plugins []Plugin
	for _, name := range artifactDirs(configDir, container) {
		path := filepath.Join(configDir, container, name)
		meta := ad.PluginMeta(path)
		hash, files := hashArtifactDir(path)
		plugins = append(plugins, Plugin{
			Name:        name,
			Version:     meta.Version,
			Path:        path,
			Description: meta.Description,
			Hash:        hash,
			fileHashes:  files,
		})
	}
	return plugins
}

// folderMarketplaces lists marketplaces by folder layout, for config dirs
// without a registry, reading each catalog for the plugins it offers.
func folderMarketplaces(ad harness.Adapter, configDir, container string) []Marketplace {
	var marketplaces []Marketplace
	for _, name := range artifactDirs(configDir, container) {
		path := filepath.Join(configDir, container, name)
		hash, files := hashArtifactDir(path)
		marketplaces = append(marketplaces, Marketplace{
			Name:        name,
			Path:        path,
			PluginNames: ad.MarketplaceMeta(path).PluginNames,
			Hash:        hash,
			fileHashes:  files,
		})
	}
	return marketplaces
}

// registryPlugins cooks the registry's installed plugins into the discovery
// listing, with the enabled state and each install's metadata.
func registryPlugins(ad harness.Adapter, reg harness.Registry) []Plugin {
	plugins := make([]Plugin, 0, len(reg.InstalledPlugins))
	for key, installs := range reg.InstalledPlugins {
		name, marketplace, _ := strings.Cut(key, "@")
		version, installPath := "", ""
		if len(installs) > 0 {
			// The last entry is the most recent install of the plugin.
			version = installs[len(installs)-1].Version
			installPath = installs[len(installs)-1].InstallPath
		}
		hash, files := hashArtifactDir(installPath)
		plugins = append(plugins, Plugin{
			Name:        name,
			Marketplace: marketplace,
			Version:     version,
			Enabled:     reg.EnabledPlugins[key],
			Path:        installPath,
			Description: ad.PluginMeta(installPath).Description,
			Hash:        hash,
			fileHashes:  files,
		})
	}

	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Name != plugins[j].Name {
			return plugins[i].Name < plugins[j].Name
		}
		return plugins[i].Marketplace < plugins[j].Marketplace
	})
	return plugins
}

// registryMarketplaces cooks the registry's known marketplaces into the
// discovery listing.
func registryMarketplaces(ad harness.Adapter, reg harness.Registry) []Marketplace {
	marketplaces := make([]Marketplace, 0, len(reg.KnownMarketplaces))
	for name, entry := range reg.KnownMarketplaces {
		hash, files := hashArtifactDir(entry.InstallLocation)
		marketplaces = append(marketplaces, Marketplace{
			Name:        name,
			Source:      entry.SourceLabel(),
			Path:        entry.InstallLocation,
			PluginNames: ad.MarketplaceMeta(entry.InstallLocation).PluginNames,
			Hash:        hash,
			fileHashes:  files,
		})
	}

	sort.Slice(marketplaces, func(i, j int) bool {
		return marketplaces[i].Name < marketplaces[j].Name
	})
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
		if ad, ok := harness.ForMarker(name); ok {
			*found = append(*found, collectConfigDir(path, ad))
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
