package doctor

import (
	"agentic-developer/internal/discovery"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxSkillLines is the house-style cap on SKILL.md (see PHILOSOPHY.md): the
// file is an index, not an encyclopedia, and everything beyond an index
// belongs in references/ or assets/.
const maxSkillLines = 200

// checkSkills validates every skill of a config dir. These checks apply to
// every harness: all of them load skills from a SKILL.md whose frontmatter
// carries the metadata.
func checkSkills(dir discovery.ConfigDir) []Finding {
	var findings []Finding
	for _, s := range dir.Skills {
		findings = append(findings, checkSkill(s)...)
	}
	return findings
}

// checkSkill validates one skill folder: SKILL.md exists, has a frontmatter
// block with the fields the harness needs, and respects the line cap.
func checkSkill(s discovery.Skill) []Finding {
	file := filepath.Join(s.Path, "SKILL.md")
	raw, err := os.ReadFile(file)
	if err != nil {
		return []Finding{{
			Severity: Error,
			Check:    "skill-md-missing",
			Path:     s.Path,
			Message:  fmt.Sprintf("skill %q has no SKILL.md, so it can never load", s.Name),
			FixHint:  "create SKILL.md with a frontmatter block (name, description), or delete the folder",
		}}
	}

	var findings []Finding

	if meta := discovery.ParseFrontmatter(file); meta == nil {
		findings = append(findings, Finding{
			Severity: Error,
			Check:    "skill-frontmatter-missing",
			Path:     file,
			Message:  fmt.Sprintf("SKILL.md of %q has no frontmatter block, so the harness cannot read its metadata", s.Name),
			FixHint:  "open the file with a --- fenced block holding name and description",
		})
	} else {
		// Without a description the model has nothing to decide with, so the
		// skill exists but never triggers.
		if strings.TrimSpace(meta["description"]) == "" {
			findings = append(findings, Finding{
				Severity: Error,
				Check:    "skill-description-missing",
				Path:     file,
				Message:  fmt.Sprintf("SKILL.md of %q has no description in its frontmatter, so the skill never triggers", s.Name),
				FixHint:  "add a description with explicit keywords for when to use the skill",
			})
		}
		if strings.TrimSpace(meta["name"]) == "" {
			findings = append(findings, Finding{
				Severity: Warning,
				Check:    "skill-name-missing",
				Path:     file,
				Message:  fmt.Sprintf("SKILL.md of %q has no name in its frontmatter", s.Name),
				FixHint:  "add a name matching the folder, in kebab-case",
			})
		}
	}

	if lines := countLines(raw); lines > maxSkillLines {
		findings = append(findings, Finding{
			Severity: Warning,
			Check:    "skill-md-too-long",
			Path:     file,
			Message:  fmt.Sprintf("SKILL.md of %q is %d lines long (house style caps it at %d)", s.Name, lines, maxSkillLines),
			FixHint:  "move knowledge into references/ or assets/ and keep SKILL.md an index",
		})
	}

	return findings
}

// countLines counts the lines of raw, counting a trailing line without a
// final newline as one more line.
func countLines(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	lines := bytes.Count(raw, []byte("\n"))
	if raw[len(raw)-1] != '\n' {
		lines++
	}
	return lines
}
