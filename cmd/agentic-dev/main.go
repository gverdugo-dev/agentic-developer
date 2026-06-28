package main

import (
	"agentic-developer/internal/cli"
	"log/slog"
	"os"
)

// agentic-dev <verb> <artifact> <name> [scope] [harness]
//   verb:     new | delete
//   artifact: skill | plugin | plugin-marketplace
//   scope:    project (default, current dir) | local (whole machine, home)
//   harness:  claude | codex | opencode (default: auto-detected from folders)
//
// agentic-dev new skill my-new-skill
// agentic-dev new plugin my-new-plugin local
// agentic-dev new skill my-new-skill project opencode
// agentic-dev delete skill my-new-skill

func main() {
	if err := cli.Run(os.Args); err != nil {
		slog.Error("agentic-dev failed", "err", err)
		os.Exit(1)
	}
}
