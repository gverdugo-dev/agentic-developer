// Command adev scaffolds and removes AI coding-agent artifacts (skills,
// plugins, plugin-marketplaces) for a detected or explicit harness, and
// installs its own bundled skills into a harness with `adev setup`.
//
// Usage:
//
//	adev <verb> <artifact> <name> [scope] [harness] [--force]
//	adev setup [harness]
//
//	verb:     new | delete
//	artifact: skill | plugin | plugin-marketplace
//	scope:    project (default, current dir) | local (whole machine, home)
//	harness:  claude | codex | opencode (default: auto-detected from folders)
//	--force:  overwrite an existing artifact instead of refusing
//
// Examples:
//
//	adev new skill my-new-skill
//	adev new plugin my-new-plugin local
//	adev new skill my-new-skill project opencode
//	adev new skill my-new-skill --force
//	adev delete skill my-new-skill
//	adev setup          # install adev's skills into the detected harness (home)
//	adev setup claude   # ... into a specific harness
package main

import (
	"agentic-developer/internal/cli"
	"log/slog"
	"os"
)

func main() {
	if err := cli.Run(os.Args); err != nil {
		slog.Error("adev failed", "err", err)
		os.Exit(1)
	}
}
