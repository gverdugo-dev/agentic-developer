package discovery

import (
	"agentic-developer/internal/harness"
	"agentic-developer/internal/scaffolding"
)

// This file keeps the pre-adapter registry API compiling. The Claude
// registry reading lives in the Claude adapter now (internal/harness);
// callers written against discovery keep working through these aliases.

// ClaudeRegistry is the raw plugin registry of one Claude config dir.
//
// Deprecated: use the Claude adapter's Registry (internal/harness).
type ClaudeRegistry = harness.Registry

// PluginInstall is one install record of a registry plugin.
//
// Deprecated: use harness.PluginInstall.
type PluginInstall = harness.PluginInstall

// RegisteredMarketplace is one known_marketplaces.json entry.
//
// Deprecated: use harness.RegisteredMarketplace.
type RegisteredMarketplace = harness.RegisteredMarketplace

// ReadClaudeRegistry reads the raw plugin registry of a Claude config dir.
//
// Deprecated: use the Claude adapter's Registry (internal/harness).
func ReadClaudeRegistry(configDir string) ClaudeRegistry {
	ad, _ := harness.ForID(scaffolding.Claude)
	return ad.Registry(configDir)
}
