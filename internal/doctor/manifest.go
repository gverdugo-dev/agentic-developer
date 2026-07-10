package doctor

import (
	"agentic-developer/internal/discovery"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// checkManifests validates the plugin.json / marketplace.json manifest of
// every plugin and marketplace of a Claude config dir that is materialized
// on disk. Folders that do not exist are not this check's business: the
// registry checks flag those.
func checkManifests(dir discovery.ConfigDir) []Finding {
	var findings []Finding
	for _, p := range dir.Plugins {
		findings = append(findings, checkManifest("plugin", p.Name, p.Path, "plugin.json")...)
	}
	for _, m := range dir.Marketplaces {
		findings = append(findings, checkManifest("marketplace", m.Name, m.Path, "marketplace.json")...)
	}
	return findings
}

// checkManifest validates one .claude-plugin manifest: it must exist, parse
// as JSON, and carry a name.
func checkManifest(kind, name, dir, manifestName string) []Finding {
	if dir == "" || !isDir(dir) {
		return nil
	}

	file := filepath.Join(dir, ".claude-plugin", manifestName)
	raw, err := os.ReadFile(file)
	if err != nil {
		return []Finding{{
			Severity: Error,
			Check:    kind + "-manifest-missing",
			Path:     dir,
			Message:  fmt.Sprintf("%s %q has no .claude-plugin/%s, so the harness cannot load it", kind, name, manifestName),
			FixHint:  fmt.Sprintf("create .claude-plugin/%s with at least a name field", manifestName),
		}}
	}

	var manifest struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return []Finding{{
			Severity: Error,
			Check:    kind + "-manifest-invalid",
			Path:     file,
			Message:  fmt.Sprintf("%s of %s %q is not valid JSON: %v", manifestName, kind, name, err),
			FixHint:  "fix the JSON syntax",
		}}
	}

	if strings.TrimSpace(manifest.Name) == "" {
		return []Finding{{
			Severity: Warning,
			Check:    kind + "-name-missing",
			Path:     file,
			Message:  fmt.Sprintf("%s of %s %q has no name field", manifestName, kind, name),
			FixHint:  "add a name matching the folder, in kebab-case",
		}}
	}

	return nil
}
