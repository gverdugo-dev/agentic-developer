package adevfile

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/scaffolding"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This file builds a manifest out of the discovered reality: what `adev
// export` writes. Scope classification is shared with the sync diff, so an
// exported manifest always syncs clean against the state it was taken from.

// FromScan builds the manifest describing dirs, the result of a discovery
// scan rooted at root. Only two kinds of config dir enter the manifest: the
// user-level ones (~/.claude, user scope) and the ones sitting directly at
// root (./.claude, project scope). Config dirs nested deeper belong to other
// projects, so they are returned in skipped instead of silently claimed.
func FromScan(dirs []discovery.ConfigDir, root string) (File, []string, error) {
	file := File{Version: CurrentVersion, Harnesses: map[string]Harness{}}

	var skipped []string
	for _, dir := range dirs {
		scope, err := classifyScope(dir, root)
		if err != nil {
			return File{}, nil, err
		}
		if scope == "" {
			skipped = append(skipped, dir.Path)
			continue
		}

		label := scaffolding.AIHarnesses[dir.Harness]
		entry := entryFrom(dir)
		harness := file.Harnesses[label]
		switch scope {
		case ScopeUser:
			harness.User = &entry
		case ScopeProject:
			harness.Project = &entry
		}
		file.Harnesses[label] = harness
	}

	return file, skipped, nil
}

// classifyScope resolves which manifest scope a discovered config dir maps
// to: "user" when it lives directly in the user's home, "project" when it
// sits directly at root, and "" (out of scope) anywhere else.
func classifyScope(dir discovery.ConfigDir, root string) (string, error) {
	marker := scaffolding.MarkerFor(dir.Harness)

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve the home dir: %w", err)
	}
	if dir.Path == filepath.Join(home, marker) {
		return ScopeUser, nil
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if dir.Path == filepath.Join(abs, marker) {
		return ScopeProject, nil
	}

	return "", nil
}

// entryFrom flattens one config dir's discovered artifacts into a manifest
// entry. Discovery already sorts each category, so the entry (and the diff
// built on it) stays deterministic.
func entryFrom(dir discovery.ConfigDir) Entry {
	var entry Entry
	for _, skill := range dir.Skills {
		entry.Skills = append(entry.Skills, skill.Name)
	}
	for _, plugin := range dir.Plugins {
		entry.Plugins = append(entry.Plugins, pluginKey(plugin))
	}
	for _, mkt := range dir.Marketplaces {
		entry.Marketplaces = append(entry.Marketplaces, Marketplace{
			Name:   mkt.Name,
			Source: addableSource(mkt.Source),
		})
	}
	return entry
}

// pluginKey is the manifest identity of one discovered plugin:
// "name@marketplace" for registry plugins, the bare name for folder plugins.
func pluginKey(p discovery.Plugin) string {
	if p.Marketplace != "" {
		return p.Name + "@" + p.Marketplace
	}
	return p.Name
}

// addableSource turns discovery's human source label ("github owner/repo",
// "directory /path") back into the coordinate the harness CLI accepts as an
// add source ("owner/repo", "/path"). Labels without a known kind prefix
// (folder-based marketplaces report none) yield "", which the sync diff
// treats as not addable.
func addableSource(label string) string {
	kind, rest, found := strings.Cut(label, " ")
	if !found {
		return ""
	}
	switch kind {
	case "github", "git", "directory":
		return rest
	}
	return ""
}
