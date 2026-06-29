// Command adev scaffolds and removes AI coding-agent artifacts (skills,
// plugins, plugin-marketplaces) for a detected or explicit harness, and
// installs its own bundled skills into a harness with `adev setup`.
//
// Usage:
//
//	adev new <artifact> <name> [--scope s] [--harness h] [--force]
//	adev delete <artifact> <name> [--scope s] [--harness h]
//	adev setup [harness]
//	adev update
//	adev version
//
//	artifact:  skill | plugin | plugin-marketplace
//	--scope:   project (default, current dir) | local (whole machine, home)
//	--harness: claude | codex | opencode (default: auto-detected from folders)
//	--force:   overwrite an existing artifact instead of refusing (new only)
//
// Flags may appear before, after, or between the positional arguments.
//
// Examples:
//
//	adev new skill my-new-skill
//	adev new plugin my-new-plugin --scope local
//	adev new skill my-new-skill --harness opencode
//	adev new skill my-new-skill --force
//	adev delete skill my-new-skill
//	adev setup          # install adev's skills into the detected harness (home)
//	adev setup claude   # ... into a specific harness
//	adev update         # self-update to the latest release
//	adev version        # print the version
package main

import (
	"agentic-developer/internal/cli"
	"os"
)

func main() {
	if err := cli.Run(os.Args); err != nil {
		cli.RenderError(err)
		os.Exit(1)
	}
}
