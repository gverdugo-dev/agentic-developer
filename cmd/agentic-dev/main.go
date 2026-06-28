package main

import (
	"agentic-developer/internal/cli"
	"agentic-developer/internal/scaffolding"
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

	slog.Info("Starting aplications")
	config := scaffolding.LoadConfig()

	args, err := cli.NewArgsBody(os.Args)
	if err != nil {
		slog.Error("Error reading agentic-dev arguments")
		os.Exit(1)
	}

	artifactScaffold := scaffolding.GetScaffoldByArtifactKey(config, args.Artifact)

	slog.Info("parsed command", "artifact", args.Artifact, "scaffold", artifactScaffold)
	// scaffolding.ApplyConfig(artifactScaffold, args.ArtifactName, ".")
}
