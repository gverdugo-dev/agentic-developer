package harness

import "agentic-developer/internal/scaffolding"

// codexAdapter covers OpenAI Codex. Codex is folder-based: it loads skills
// from skills/ (the shared SKILL.md standard) and reusable prompts from
// prompts/ (each a markdown file that becomes a custom command), and takes
// its instructions from AGENTS.md, the cross-tool open standard (a global
// one inside ~/.codex and a per-project one at the repo root). It has no
// plugin system, no marketplace concept, no install registry, and no CLI
// that owns any of that, so the adapter reports exactly none of it.
type codexAdapter struct{}

func (codexAdapter) ID() scaffolding.AIHarness { return scaffolding.Codex }
func (a codexAdapter) Name() string            { return scaffolding.AIHarnesses[a.ID()] }
func (a codexAdapter) Marker() string          { return scaffolding.MarkerFor(a.ID()) }

// Containers: skills plus the prompts folder. No plugins, no marketplaces.
func (codexAdapter) Containers() []Container {
	return []Container{
		{Kind: KindSkill, Dir: "skills"},
		{Kind: KindPrompt, Dir: "prompts"},
	}
}

// InstructionFiles: AGENTS.md, inside the config dir (~/.codex/AGENTS.md is
// the global guidance) or next to it (the project root's AGENTS.md).
func (codexAdapter) InstructionFiles(configDir string) []string {
	return existingFiles(configDir, "AGENTS.md")
}

// Registry: Codex tracks no install state beyond the folders themselves.
func (codexAdapter) Registry(string) Registry { return Registry{} }

// PluginMeta: Codex has no plugins.
func (codexAdapter) PluginMeta(string) PluginMeta { return PluginMeta{} }

// MarketplaceMeta: Codex has no marketplaces.
func (codexAdapter) MarketplaceMeta(string) MarketplaceMeta { return MarketplaceMeta{} }

// ValidatePlugin: nothing to validate, Codex loads no plugin manifests.
func (codexAdapter) ValidatePlugin(string, string) []Issue { return nil }

// ValidateMarketplace: nothing to validate.
func (codexAdapter) ValidateMarketplace(string, string) []Issue { return nil }

// ValidateRegistry: no registry, nothing to cross-reference.
func (codexAdapter) ValidateRegistry(string) []Issue { return nil }

// Operations: no CLI owns a registry, so there is nothing to execute.
func (codexAdapter) Operations() Operations { return nil }
