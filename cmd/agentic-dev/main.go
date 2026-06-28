package main

import (
	"log/slog"

	"agentic-developer/internal/scaffolding"
)

func main() {

	slog.Info("Starting aplications")
	config := scaffolding.LoadConfig()

	slog.Info("config loaded", "skill", config.Structures)
}
