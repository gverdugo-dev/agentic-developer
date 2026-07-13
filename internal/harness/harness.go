// Package harness makes every supported AI harness a first-class citizen
// behind one adapter interface. An adapter describes everything adev needs
// to know about a harness: the marker directory that identifies it, which
// artifact kinds live in which folders, how to read its install registry
// (when it has one), how to validate its manifests, and how to execute
// registry operations (when it has a CLI that owns them).
//
// Adding a new harness (gemini-cli, cursor) is implementing Adapter and
// appending it to the adapter list below; discovery, the doctor and the TUI
// consume adapters and never branch on a concrete harness themselves.
//
// Adapters are honest about capability: a harness without a registry returns
// a zero Registry, one without a managing CLI returns nil Operations, and
// validators only exist for the manifests the harness actually loads.
package harness

import (
	"agentic-developer/internal/scaffolding"
	"os"
	"path/filepath"
)

// Kind is an artifact kind a harness can hold.
type Kind int

// The artifact kinds. Skills, plugins and marketplaces match the scaffolder's
// artifacts; prompts cover harnesses that load reusable prompt files from a
// folder (Codex).
const (
	KindSkill Kind = iota
	KindPrompt
	KindPlugin
	KindMarketplace
)

// Container says where one artifact kind lives inside a config dir.
type Container struct {
	Kind Kind
	// Dir is the folder holding the artifacts, relative to the config dir.
	Dir string
}

// PluginMeta is the metadata a plugin's own manifest provides, whatever file
// that is for the harness (plugin.json for Claude, package.json for
// opencode).
type PluginMeta struct {
	Name        string
	Version     string
	Description string
}

// MarketplaceMeta is the metadata a marketplace's own manifest provides.
type MarketplaceMeta struct {
	Name string
	// PluginNames is the catalog of plugins the marketplace offers.
	PluginNames []string
}

// PluginInstall is one install record of a registry plugin: the version and
// where the harness materialized it.
type PluginInstall struct {
	Version     string `json:"version"`
	InstallPath string `json:"installPath"`
}

// RegisteredMarketplace is one registry marketplace entry: the kind of
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

// Registry is the raw install registry of one config dir, for harnesses that
// track installed/enabled state beyond the folder layout (Claude). Each map
// is nil when its file is missing or unparsable; HasInstalled and
// HasMarketplaces distinguish "readable registry" from "no registry", which
// is what decides the fallback to the folder listing. The zero value means
// the harness has no registry at all.
type Registry struct {
	// InstalledPlugins maps "name@marketplace" to its install records; the
	// last record is the most recent install.
	InstalledPlugins map[string][]PluginInstall
	// KnownMarketplaces maps each registered marketplace name to its source.
	KnownMarketplaces map[string]RegisteredMarketplace
	// EnabledPlugins maps "name@marketplace" to its enabled state.
	EnabledPlugins map[string]bool
	// HasInstalled and HasMarketplaces report whether the corresponding
	// registry file was present and parsable.
	HasInstalled    bool
	HasMarketplaces bool
}

// IssueSeverity ranks a validation issue. IssueError is the zero value:
// breakage first, matching the doctor's ordering.
type IssueSeverity int

// The severities. Errors break the artifact at runtime; warnings are
// hygiene or house-style problems.
const (
	IssueError IssueSeverity = iota
	IssueWarning
)

// Issue is one problem a validator found: what fired, where, how bad it is,
// and what to do about it. The doctor maps issues to its findings one to
// one.
type Issue struct {
	Severity IssueSeverity
	// Check is the stable identifier of the check that fired.
	Check   string
	Path    string
	Message string
	FixHint string
}

// Operations executes the registry mutations of a harness that has a CLI
// owning that registry. Every operation returns the CLI's trimmed output.
type Operations interface {
	InstallPlugin(key string) (string, error)
	UninstallPlugin(key string) (string, error)
	SetPluginEnabled(key string, enabled bool) (string, error)
	AddMarketplace(source string) (string, error)
	RemoveMarketplace(name string) (string, error)
}

// Adapter is everything adev knows about one harness.
type Adapter interface {
	// ID is the harness's typed identity in the scaffolding domain.
	ID() scaffolding.AIHarness
	// Name is the display name ("claude", "codex", "opencode").
	Name() string
	// Marker is the config dir name that identifies the harness (".claude").
	Marker() string
	// Containers lists where each artifact kind the harness supports lives.
	Containers() []Container
	// InstructionFiles returns the harness's instruction/config files found
	// in or next to configDir (CLAUDE.md, AGENTS.md, opencode.json), for
	// awareness in listings. Only files that exist are returned.
	InstructionFiles(configDir string) []string
	// Registry reads the config dir's install registry. Harnesses without
	// one return the zero Registry.
	Registry(configDir string) Registry
	// PluginMeta reads the manifest of the plugin living in dir, returning
	// the zero value when there is none (or the harness has no plugins).
	PluginMeta(dir string) PluginMeta
	// MarketplaceMeta reads the manifest of the marketplace living in dir.
	MarketplaceMeta(dir string) MarketplaceMeta
	// ValidatePlugin checks the manifest of one plugin that is materialized
	// on disk. Folders that do not exist are the registry checks' business.
	ValidatePlugin(name, dir string) []Issue
	// ValidateMarketplace checks the manifest of one marketplace on disk.
	ValidateMarketplace(name, dir string) []Issue
	// ValidateRegistry cross-references the config dir's registry against
	// the disk: ghost plugins, missing caches, orphaned caches, dead
	// sources. Harnesses without a registry return nothing.
	ValidateRegistry(configDir string) []Issue
	// Operations executes registry mutations, nil when the harness has no
	// CLI owning a registry.
	Operations() Operations
}

// adapters holds every supported harness in detection priority order:
// Claude, Codex, opencode. The order must match
// scaffolding.HarnessesInOrder, the single source of truth for priority.
var adapters = []Adapter{claudeAdapter{}, codexAdapter{}, opencodeAdapter{}}

// All returns every adapter in detection priority order.
func All() []Adapter {
	out := make([]Adapter, len(adapters))
	copy(out, adapters)
	return out
}

// ForID returns the adapter of one harness identity.
func ForID(id scaffolding.AIHarness) (Adapter, bool) {
	for _, ad := range adapters {
		if ad.ID() == id {
			return ad, true
		}
	}
	return nil, false
}

// ForMarker returns the adapter whose config dir marker is name (".claude"
// resolves to the Claude adapter).
func ForMarker(name string) (Adapter, bool) {
	for _, ad := range adapters {
		if ad.Marker() == name {
			return ad, true
		}
	}
	return nil, false
}

// isDir reports whether path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// existingFiles returns, of the given file names, those that exist either
// inside configDir or next to it (in its parent), in that order. This is how
// adapters report the instruction files a harness honors: the user-level
// file lives inside the config dir (~/.claude/CLAUDE.md, ~/.codex/AGENTS.md)
// and the project-level one next to it (project/CLAUDE.md, project/AGENTS.md).
func existingFiles(configDir string, names ...string) []string {
	var found []string
	for _, name := range names {
		for _, dir := range []string{configDir, filepath.Dir(configDir)} {
			path := filepath.Join(dir, name)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				found = append(found, path)
			}
		}
	}
	return found
}
