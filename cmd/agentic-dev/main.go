// Command agentic-dev scaffolds and removes AI coding-agent artifacts (skills,
// plugins, plugin-marketplaces) for a detected or explicit harness.
//
// Usage:
//
//	agentic-dev <verb> <artifact> <name> [scope] [harness]
//
//	verb:     new | delete
//	artifact: skill | plugin | plugin-marketplace
//	scope:    project (default, current dir) | local (whole machine, home)
//	harness:  claude | codex | opencode (default: auto-detected from folders)
//
// Examples:
//
//	agentic-dev new skill my-new-skill
//	agentic-dev new plugin my-new-plugin local
//	agentic-dev new skill my-new-skill project opencode
//	agentic-dev delete skill my-new-skill
package main

import (
	"agentic-developer/internal/cli"
	"log/slog"
	"os"
)

func main() {
	if err := cli.Run(os.Args); err != nil {
		slog.Error("agentic-dev failed", "err", err)
		os.Exit(1)
	}
}
