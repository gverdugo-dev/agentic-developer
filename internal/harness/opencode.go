package harness

import (
	"agentic-developer/internal/scaffolding"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// opencodeAdapter covers opencode. opencode loads skills from skills/ (the
// shared SKILL.md standard) and TypeScript plugins: each plugin is a folder
// with an index.ts entry point and a package.json manifest (the layout adev
// scaffolds). Its configuration is opencode.json (at the project root, next
// to the config dir) and it also honors AGENTS.md instructions. It has no
// marketplace concept, no install registry, and no CLI owning one.
type opencodeAdapter struct{}

func (opencodeAdapter) ID() scaffolding.AIHarness { return scaffolding.Opencode }
func (a opencodeAdapter) Name() string            { return scaffolding.AIHarnesses[a.ID()] }
func (a opencodeAdapter) Marker() string          { return scaffolding.MarkerFor(a.ID()) }

// Containers: skills and TypeScript plugins. No marketplaces.
func (opencodeAdapter) Containers() []Container {
	return []Container{
		{Kind: KindSkill, Dir: "skills"},
		{Kind: KindPlugin, Dir: "plugins"},
	}
}

// InstructionFiles: the opencode.json config and AGENTS.md, in or next to
// the config dir.
func (opencodeAdapter) InstructionFiles(configDir string) []string {
	return existingFiles(configDir, "opencode.json", "AGENTS.md")
}

// Registry: opencode tracks no install state beyond the folders themselves.
func (opencodeAdapter) Registry(string) Registry { return Registry{} }

// PluginMeta reads the plugin's package.json: a TS plugin's manifest.
func (opencodeAdapter) PluginMeta(dir string) PluginMeta {
	var meta PluginMeta
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
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

// MarketplaceMeta: opencode has no marketplaces.
func (opencodeAdapter) MarketplaceMeta(string) MarketplaceMeta { return MarketplaceMeta{} }

// ValidatePlugin checks a TS plugin that is materialized on disk: it needs
// an index.ts entry point to load at all, and its package.json (when
// present) must parse.
func (opencodeAdapter) ValidatePlugin(name, dir string) []Issue {
	if dir == "" || !isDir(dir) {
		return nil
	}

	var issues []Issue

	if _, err := os.Stat(filepath.Join(dir, "index.ts")); err != nil {
		issues = append(issues, Issue{
			Severity: IssueError,
			Check:    "plugin-entry-missing",
			Path:     dir,
			Message:  fmt.Sprintf("plugin %q has no index.ts, so opencode cannot load it", name),
			FixHint:  "create index.ts exporting the plugin, or delete the folder",
		})
	}

	manifest := filepath.Join(dir, "package.json")
	if raw, err := os.ReadFile(manifest); err == nil {
		if !json.Valid(raw) {
			issues = append(issues, Issue{
				Severity: IssueError,
				Check:    "plugin-package-invalid",
				Path:     manifest,
				Message:  fmt.Sprintf("package.json of plugin %q is not valid JSON", name),
				FixHint:  "fix the JSON syntax",
			})
		}
	}

	return issues
}

// ValidateMarketplace: nothing to validate, opencode has no marketplaces.
func (opencodeAdapter) ValidateMarketplace(string, string) []Issue { return nil }

// ValidateRegistry: no registry, nothing to cross-reference.
func (opencodeAdapter) ValidateRegistry(string) []Issue { return nil }

// Operations: no CLI owns a registry, so there is nothing to execute.
func (opencodeAdapter) Operations() Operations { return nil }
