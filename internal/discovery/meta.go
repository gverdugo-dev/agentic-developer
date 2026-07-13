package discovery

import (
	"os"
	"path/filepath"
	"strings"
)

// This file parses SKILL.md frontmatter, the one artifact metadata format
// shared by every harness (the open skills standard), so every component can
// show a real preview. Harness-specific manifests (plugin.json,
// package.json, marketplace.json) are the adapters' business (see
// internal/harness). Parsing is best-effort: a missing or malformed file
// just yields empty metadata, never an error.

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
