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
	"agentic-developer/internal/harness"
	"encoding/json"
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
//
// The skill checks are the doctor's own: SKILL.md is the one standard every
// harness shares. Everything harness-specific (manifest shapes, registry
// cross-references) comes from the dir's adapter, so the doctor never
// branches on a concrete harness.
func Check(dirs []discovery.ConfigDir) []Finding {
	var findings []Finding
	for _, dir := range dirs {
		findings = append(findings, checkSkills(dir)...)

		ad, ok := harness.ForID(dir.Harness)
		if !ok {
			continue
		}
		for _, p := range dir.Plugins {
			findings = append(findings, toFindings(ad.ValidatePlugin(p.Name, p.Path))...)
		}
		for _, m := range dir.Marketplaces {
			findings = append(findings, toFindings(ad.ValidateMarketplace(m.Name, m.Path))...)
		}
		findings = append(findings, toFindings(ad.ValidateRegistry(dir.Path))...)
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

// toFindings maps an adapter's validation issues to doctor findings, one to
// one.
func toFindings(issues []harness.Issue) []Finding {
	findings := make([]Finding, 0, len(issues))
	for _, issue := range issues {
		severity := Error
		if issue.Severity == harness.IssueWarning {
			severity = Warning
		}
		findings = append(findings, Finding{
			Severity: severity,
			Check:    issue.Check,
			Path:     issue.Path,
			Message:  issue.Message,
			FixHint:  issue.FixHint,
		})
	}
	return findings
}
