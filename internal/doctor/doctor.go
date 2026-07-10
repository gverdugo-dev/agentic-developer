// Package doctor runs health checks over discovered AI harness artifacts and
// reports prioritized findings. Broken artifacts fail silently at runtime: a
// skill without SKILL.md never triggers, a plugin enabled in settings but
// missing from the registry is a ghost, a marketplace pointing at a deleted
// directory rots. One report of everything wrong turns those surprises into
// a checklist.
//
// The package does no discovery of its own: Check operates on the config
// dirs package discovery already found, so the doctor sees exactly what the
// rest of adev sees.
package doctor

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/scaffolding"
	"encoding/json"
	"os"
	"sort"
)

// Severity ranks a finding. Errors break the artifact at runtime; warnings
// are house-style or hygiene problems. Error is the zero value so the
// default sort puts the breakage first.
type Severity int

// The severities, in priority order.
const (
	Error Severity = iota
	Warning
)

// String names the severity for reports.
func (s Severity) String() string {
	if s == Error {
		return "error"
	}
	return "warning"
}

// MarshalJSON encodes the severity as its name, so JSON consumers read
// "error"/"warning" instead of an opaque number.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// Finding is one detected problem: what is wrong, where, how bad it is, and
// what to do about it.
type Finding struct {
	Severity Severity `json:"severity"`
	// Check is the stable identifier of the check that fired, so scripts can
	// filter findings by kind.
	Check string `json:"check"`
	// Path is the artifact or file the finding is about.
	Path    string `json:"path"`
	Message string `json:"message"`
	FixHint string `json:"fixHint,omitempty"`
}

// Check runs every health check over the discovered config dirs and returns
// the findings sorted by severity (errors first), then path, then check, so
// the report reads as a prioritized checklist. A healthy tree yields nil.
func Check(dirs []discovery.ConfigDir) []Finding {
	var findings []Finding
	for _, dir := range dirs {
		findings = append(findings, checkSkills(dir)...)
		// plugin.json, marketplace.json and the plugin registry are Claude
		// concepts; the other harnesses have neither.
		if dir.Harness == scaffolding.Claude {
			findings = append(findings, checkManifests(dir)...)
			findings = append(findings, checkRegistry(dir)...)
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Check < b.Check
	})
	return findings
}

// isDir reports whether path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
