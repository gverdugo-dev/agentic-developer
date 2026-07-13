// Package clean collects removal candidates from the discovered AI harness
// artifacts: stale plugin cache versions, orphaned cache folders, dead
// marketplaces and doctor-flagged broken artifacts. Caches accumulate old
// versions and orphans silently, and doctor findings need an actuator;
// deleting dozens of items one by one does not scale.
//
// The package decides nothing on its own state: Collect derives every
// candidate from what discovery already found and what the doctor already
// flagged, so clean sees exactly what the rest of adev sees. Registries are
// read through each harness's adapter (internal/harness); harnesses without
// one return the zero Registry and contribute no registry candidates. Apply
// executes one candidate: registry-owned entries go through the claude CLI
// (the Claude adapter's executor, via manage's helpers), plain orphaned
// folders are deleted from disk.
package clean

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/doctor"
	"agentic-developer/internal/harness"
	"agentic-developer/internal/manage"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Action is how a candidate gets removed. Anything recorded in Claude's
// plugin registry must go through the claude CLI, which owns that state;
// only folders nothing claims are deleted directly.
type Action int

// The removal actions.
const (
	// ActionRemoveDir deletes the candidate's path from disk. Only for
	// folders no registry entry claims.
	ActionRemoveDir Action = iota
	// ActionUninstall runs `claude plugin uninstall <Arg>`.
	ActionUninstall
	// ActionRemoveMarketplace runs `claude plugin marketplace remove <Arg>`.
	ActionRemoveMarketplace
)

// String names the action for reports.
func (a Action) String() string {
	switch a {
	case ActionUninstall:
		return "uninstall-plugin"
	case ActionRemoveMarketplace:
		return "remove-marketplace"
	default:
		return "remove-dir"
	}
}

// MarshalJSON encodes the action as its name, so JSON consumers read
// "remove-dir"/"uninstall-plugin"/"remove-marketplace" instead of a number.
func (a Action) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON decodes an action from its name, so candidates round-trip
// through the JSON the CLI emits.
func (a *Action) UnmarshalJSON(raw []byte) error {
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return err
	}
	switch name {
	case ActionRemoveDir.String():
		*a = ActionRemoveDir
	case ActionUninstall.String():
		*a = ActionUninstall
	case ActionRemoveMarketplace.String():
		*a = ActionRemoveMarketplace
	default:
		return fmt.Errorf("unknown clean action %q", name)
	}
	return nil
}

// Candidate is one removable item: what kind of dead weight it is, where it
// lives, why it is removable, how much disk it reclaims, and how to remove
// it. Arg carries the plugin key or marketplace name for the claude CLI
// actions.
type Candidate struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
	// Size is the reclaimable size in bytes, 0 when nothing is on disk to
	// reclaim (a dead marketplace is a registry entry, not a folder).
	Size   int64  `json:"reclaimableBytes"`
	Action Action `json:"action"`
	Arg    string `json:"arg,omitempty"`
}

// The candidate kinds.
const (
	// KindStaleVersion is a cached plugin version no install record claims,
	// left behind by an update. The installed version is always kept.
	KindStaleVersion = "stale-plugin-version"
	// KindCacheOrphan is a plugin cache folder with no registry entry at all.
	KindCacheOrphan = "cache-orphan"
	// KindDeadMarketplace is a directory-source marketplace whose path no
	// longer exists.
	KindDeadMarketplace = "dead-marketplace"
	// KindBrokenArtifact is an artifact the doctor flagged as unloadable.
	KindBrokenArtifact = "broken-artifact"
)

// Collect derives the removal candidates from the discovered config dirs and
// the doctor findings computed over them. It only reads; nothing is removed
// until Apply. The result is deduplicated and sorted by kind, then path, so
// reports are deterministic.
func Collect(dirs []discovery.ConfigDir, findings []doctor.Finding) []Candidate {
	var candidates []Candidate

	for _, dir := range dirs {
		// The registry comes from the dir's harness adapter. Harnesses
		// without one (codex, opencode) return the zero Registry, so they
		// contribute no cache or marketplace candidates; the cache layout
		// walked below (plugins/cache/...) is Claude's, and only Claude has
		// a registry today.
		ad, ok := harness.ForID(dir.Harness)
		if !ok {
			continue
		}
		reg := ad.Registry(dir.Path)
		candidates = append(candidates, cacheCandidates(dir.Path, reg)...)
		candidates = append(candidates, deadMarketplaces(reg)...)
	}

	candidates = append(candidates, brokenArtifacts(dirs, findings)...)

	candidates = dedupe(candidates)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Kind != candidates[j].Kind {
			return candidates[i].Kind < candidates[j].Kind
		}
		return candidates[i].Path < candidates[j].Path
	})
	return candidates
}

// Apply removes one candidate. Registry-owned entries are delegated to the
// claude CLI, which owns installed_plugins.json and the cache; plain
// orphaned folders are deleted from disk through the guarded delete.
func Apply(c Candidate) error {
	switch c.Action {
	case ActionUninstall:
		_, err := manage.UninstallPlugin(c.Arg)
		return err
	case ActionRemoveMarketplace:
		_, err := manage.RemoveMarketplace(c.Arg)
		return err
	default:
		return manage.DeleteArtifact(c.Path)
	}
}

// cacheCandidates walks the config dir's plugin cache
// (plugins/cache/<marketplace>/<plugin>/<version>) against the registry:
// a plugin folder with no registry entry is an orphan, and inside a
// registered plugin's folder, any version dir no install record points at
// is a stale leftover. Without a readable registry there is nothing to
// compare against, so the cache is left alone.
func cacheCandidates(configDir string, reg harness.Registry) []Candidate {
	if !reg.HasInstalled {
		return nil
	}
	cacheDir := filepath.Join(configDir, "plugins", "cache")
	marketplaces, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil
	}

	var candidates []Candidate
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
			key := plugin.Name() + "@" + mkt.Name()
			pluginDir := filepath.Join(cacheDir, mkt.Name(), plugin.Name())

			installs, installed := reg.InstalledPlugins[key]
			if !installed {
				candidates = append(candidates, Candidate{
					Kind:   KindCacheOrphan,
					Path:   pluginDir,
					Reason: fmt.Sprintf("cached plugin %q has no registry entry", key),
					Size:   dirSize(pluginDir),
					Action: ActionRemoveDir,
				})
				continue
			}
			candidates = append(candidates, staleVersions(pluginDir, key, installs)...)
		}
	}
	return candidates
}

// staleVersions lists the version dirs of one registered plugin's cache
// folder that no install record points at: leftovers of previous versions.
// When no record carries an install path there is nothing safe to compare
// against, so everything is kept.
func staleVersions(pluginDir, key string, installs []harness.PluginInstall) []Candidate {
	keep := make(map[string]bool, len(installs))
	for _, install := range installs {
		if install.InstallPath != "" {
			keep[filepath.Clean(install.InstallPath)] = true
		}
	}
	if len(keep) == 0 {
		return nil
	}

	versions, err := os.ReadDir(pluginDir)
	if err != nil {
		return nil
	}

	var candidates []Candidate
	for _, version := range versions {
		if !version.IsDir() {
			continue
		}
		path := filepath.Join(pluginDir, version.Name())
		if keep[path] {
			continue
		}
		candidates = append(candidates, Candidate{
			Kind:   KindStaleVersion,
			Path:   path,
			Reason: fmt.Sprintf("cached version %q of plugin %q is not the installed one", version.Name(), key),
			Size:   dirSize(path),
			Action: ActionRemoveDir,
		})
	}
	return candidates
}

// deadMarketplaces lists the registered directory-source marketplaces whose
// path no longer exists: they read straight from that path, so every plugin
// installed from them rots. Removal goes through the claude CLI, since the
// entry lives in its registry.
func deadMarketplaces(reg harness.Registry) []Candidate {
	names := make([]string, 0, len(reg.KnownMarketplaces))
	for name := range reg.KnownMarketplaces {
		names = append(names, name)
	}
	sort.Strings(names)

	var candidates []Candidate
	for _, name := range names {
		mkt := reg.KnownMarketplaces[name]
		if mkt.Kind != "directory" || isDir(mkt.Path) {
			continue
		}
		candidates = append(candidates, Candidate{
			Kind:   KindDeadMarketplace,
			Path:   mkt.Path,
			Reason: fmt.Sprintf("marketplace %q points at directory %s, which no longer exists", name, mkt.Path),
			Action: ActionRemoveMarketplace,
			Arg:    name,
		})
	}
	return candidates
}

// brokenArtifacts turns the doctor findings whose remedy is removal into
// candidates: a skill without SKILL.md and a plugin without its manifest can
// never load, so the folder is dead weight. A registry-backed plugin is
// uninstalled through the claude CLI; a plain folder artifact is deleted
// from disk. Fixable findings (bad frontmatter, long files) are not clean's
// business, and registered marketplaces with manifest trouble are left to
// the user: removing one would drop every plugin installed from it.
func brokenArtifacts(dirs []discovery.ConfigDir, findings []doctor.Finding) []Candidate {
	plugins := make(map[string]discovery.Plugin)
	marketplaces := make(map[string]discovery.Marketplace)
	for _, dir := range dirs {
		for _, p := range dir.Plugins {
			plugins[p.Path] = p
		}
		for _, m := range dir.Marketplaces {
			marketplaces[m.Path] = m
		}
	}

	var candidates []Candidate
	for _, f := range findings {
		switch f.Check {
		case "skill-md-missing":
			candidates = append(candidates, Candidate{
				Kind:   KindBrokenArtifact,
				Path:   f.Path,
				Reason: f.Message,
				Size:   dirSize(f.Path),
				Action: ActionRemoveDir,
			})

		case "plugin-manifest-missing":
			c := Candidate{
				Kind:   KindBrokenArtifact,
				Path:   f.Path,
				Reason: f.Message,
				Size:   dirSize(f.Path),
				Action: ActionRemoveDir,
			}
			if p, ok := plugins[f.Path]; ok && p.Marketplace != "" {
				c.Action = ActionUninstall
				c.Arg = p.Name + "@" + p.Marketplace
			}
			candidates = append(candidates, c)

		case "marketplace-manifest-missing":
			if m, ok := marketplaces[f.Path]; ok && m.Source != "" {
				continue // registered: removal is the user's call
			}
			candidates = append(candidates, Candidate{
				Kind:   KindBrokenArtifact,
				Path:   f.Path,
				Reason: f.Message,
				Size:   dirSize(f.Path),
				Action: ActionRemoveDir,
			})
		}
	}
	return candidates
}

// dedupe drops candidates that would perform the same removal twice.
func dedupe(candidates []Candidate) []Candidate {
	seen := make(map[string]bool, len(candidates))
	out := candidates[:0]
	for _, c := range candidates {
		key := c.Action.String() + "\x00" + c.Path + "\x00" + c.Arg
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// dirSize sums the regular files under path. Unreadable entries are skipped:
// the size is a report detail, never worth failing over.
func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// HumanSize renders a byte count for humans (B, KB, MB, ...), shared by the
// CLI report and the TUI so both phrase sizes the same way.
func HumanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, unit := range []string{"KB", "MB", "GB", "TB"} {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%.1f PB", value/1024)
}

// isDir reports whether path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
