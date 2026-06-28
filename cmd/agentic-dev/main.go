package main

import (
	"log/slog"

	"agentic-developer/internal/scaffolding"
)

func main() {

	slog.Info("Starting aplications")
	config := scaffolding.LoadConfig()

	resources := config.Harness["claude"].Structures["skill"]
	slog.Info("config loaded", "harness", "claude", "structure", "skill", "resources", resources)

	if err := scaffolding.ApplyConfig(resources, "my-skill", "."); err != nil {
		slog.Error("failed to apply config", "err", err)
		return
	}

	slog.Info("scaffold created")
}
