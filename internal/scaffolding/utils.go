package scaffolding

import (
	"fmt"
	"os"
	"path/filepath"
)

// harnessMarkers maps each harness to the tool-specific config directory that
// uniquely identifies it in a project. AGENTS.md is deliberately NOT here: it's
// the cross-tool open standard, so its presence doesn't tell us WHICH harness.
var harnessMarkers = map[AIHarness]string{
	Claude:   ".claude",
	Codex:    ".codex",
	Opencode: ".opencode",
}

// detectionOrder fixes the priority used when several config dirs coexist, so
// detection is deterministic (map iteration order in Go is random).
var detectionOrder = []AIHarness{Claude, Codex, Opencode}

// DetectAIHarness looks for a tool-specific config directory in baseDir only —
// the same dir where the artifact will be written. It does NOT fall back to the
// user's home, so it never infers a harness from elsewhere and then scaffolds it
// into a project that didn't have one. Pass the harness explicitly to override.
func DetectAIHarness(baseDir string) (AIHarness, error) {
	if h, ok := detectIn(baseDir); ok {
		return h, nil
	}

	return 0, fmt.Errorf("no AI harness detected in %q (looked for .claude, .codex, .opencode); pass the harness explicitly", baseDir)
}

// detectIn returns the first harness whose marker directory exists under dir,
// honouring detectionOrder.
func detectIn(dir string) (AIHarness, bool) {
	for _, h := range detectionOrder {
		if isDir(filepath.Join(dir, harnessMarkers[h])) {
			return h, true
		}
	}
	return 0, false
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// artifactSubdir is the directory, inside the harness config dir, where each
// artifact type is scaffolded.
var artifactSubdir = map[Artifact]string{
	Skill:             "skills",
	Plugin:            "plugins",
	PluginMarketplace: "marketplaces",
}

// PlacementPrefix returns the sub-path, relative to the scope base dir, where
// an artifact lives for a given harness. Everything is scaffolded inside the
// harness config dir, e.g. ".claude/skills" or ".claude/plugins".
func PlacementPrefix(harness AIHarness, artifact Artifact) string {
	return filepath.Join(harnessMarkers[harness], artifactSubdir[artifact])
}

// ScopeBaseDir resolves the base directory where an artifact is scaffolded for
// a given scope: Project -> current dir, Local -> the user's home.
func ScopeBaseDir(scope Scope) (string, error) {
	switch scope {
	case Project:
		return ".", nil
	case Local:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home dir for local scope: %w", err)
		}
		return home, nil
	default:
		return "", fmt.Errorf("unhandled scope %v", scope)
	}
}

// GetScaffoldByArtifactKey returns the resource paths defined for an artifact
// under the given harness, or an error if the harness/artifact pair has no
// structures in the config.
func GetScaffoldByArtifactKey(config Config, harness AIHarness, artifact Artifact) ([]string, error) {
	harnessKey := AIHarnesses[harness]

	h, ok := config.Harness[harnessKey]
	if !ok {
		return nil, fmt.Errorf("harness %q has no structures defined", harnessKey)
	}

	key := artifactJSONKey[artifact]
	resources, ok := h.Structures[key]
	if !ok {
		return nil, fmt.Errorf("artifact %q not defined for harness %q", key, harnessKey)
	}

	return resources, nil
}
