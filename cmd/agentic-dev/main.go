package main

import (
	"agentic-developer/internal/cli"
	"log/slog"
	"os"
)

// agentic-dev new skill my-new-skill
// agentic-dev new plugin my-new-plugin
// agentic-dev new plugin-marketplace my-new-plugin-marketplace
// agentic-dev delete skill my-new-skill
// agentic-dev delete plugin my-new-plugin
// agentic-dev delete plugin-marketplace my-new-plugin-marketplace

func main() {
	if err := cli.Run(os.Args); err != nil {
		slog.Error("agentic-dev failed", "err", err)
		os.Exit(1)
	}
}
