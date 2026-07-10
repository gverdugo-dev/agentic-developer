package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// This file parses the artifacts' own metadata files, so every component can
// show a real preview: SKILL.md frontmatter for skills, plugin.json for
// plugins, marketplace.json for marketplaces. Parsing is best-effort: a
// missing or malformed file just yields empty metadata, never an error.

// ParseFrontmatter reads the YAML-ish frontmatter block of a markdown file
// (the lines between the two --- fences) into a key/value map, or nil when
// the file has no complete frontmatter block. Only simple single-line
// "key: value" pairs are read, which is all a SKILL.md needs; quoting around
// the value is stripped. Exported so the doctor checks validate skills with
// the exact same reading discovery uses.
func ParseFrontmatter(file string) map[string]string {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}

	meta := make(map[string]string)
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return meta
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v := strings.TrimSpace(value)
		v = strings.Trim(v, `"'`)
		meta[strings.ToLower(strings.TrimSpace(key))] = v
	}

	// No closing fence: not frontmatter.
	return nil
}

// skillDescription reads the description of the skill living in dir.
func skillDescription(dir string) string {
	return ParseFrontmatter(filepath.Join(dir, "SKILL.md"))["description"]
}

// pluginManifest is the relevant subset of a plugin's
// .claude-plugin/plugin.json.
type pluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// readPluginManifest parses the manifest of the plugin living in dir,
// returning the zero value when there is none.
func readPluginManifest(dir string) pluginManifest {
	var man pluginManifest
	raw, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
	if err != nil {
		return man
	}
	_ = json.Unmarshal(raw, &man)
	return man
}

// marketplaceManifest is the relevant subset of a marketplace's
// .claude-plugin/marketplace.json: the catalog of plugins it offers.
type marketplaceManifest struct {
	Name    string `json:"name"`
	Plugins []struct {
		Name string `json:"name"`
	} `json:"plugins"`
}

// marketplacePluginNames parses the catalog of the marketplace living in dir
// and returns the plugin names it offers.
func marketplacePluginNames(dir string) []string {
	raw, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "marketplace.json"))
	if err != nil {
		return nil
	}
	var man marketplaceManifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return nil
	}

	var names []string
	for _, p := range man.Plugins {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return names
}
