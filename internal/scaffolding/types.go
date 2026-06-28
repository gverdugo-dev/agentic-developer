package scaffolding

import "fmt"

type AIHarness int

const (
	Codex AIHarness = iota
	Claude
	Opencode
)

var AIHarnesses = map[AIHarness]string{
	Codex:    "codex",
	Claude:   "claude",
	Opencode: "opencode",
}

// ParseHarness resolves the CLI string (e.g. "claude") into an AIHarness, or
// errors if the string matches no known harness.
func ParseHarness(s string) (AIHarness, error) {
	for h, label := range AIHarnesses {
		if label == s {
			return h, nil
		}
	}
	return 0, fmt.Errorf("unknown harness %q", s)
}

type Verb int

const (
	New Verb = iota
	Delete
)

var Verbs = map[Verb]string{
	New:    "new",
	Delete: "delete",
}

// ParseVerb resolves the CLI string (e.g. "new") into a Verb, or errors if the
// string matches no known verb.
func ParseVerb(s string) (Verb, error) {
	for v, label := range Verbs {
		if label == s {
			return v, nil
		}
	}
	return 0, fmt.Errorf("unknown verb %q", s)
}

type Artifact int

const (
	Skill Artifact = iota
	Plugin
	PluginMarketplace
)

var Artifacts = map[Artifact]string{
	Skill:             "skill",
	Plugin:            "plugin",
	PluginMarketplace: "plugin-marketplace",
}

// artifactJSONKey maps each Artifact to its key in structures.json. It is kept
// separate from Artifacts because the CLI label (kebab-case, what the user
// types) and the JSON key (camelCase) differ for some artifacts.
var artifactJSONKey = map[Artifact]string{
	Skill:             "skill",
	Plugin:            "plugin",
	PluginMarketplace: "pluginMarketplace",
}

// ParseArtifact resolves the CLI string (e.g. "plugin-marketplace") into an
// Artifact, or errors if the string matches no known artifact.
func ParseArtifact(s string) (Artifact, error) {
	for a, label := range Artifacts {
		if label == s {
			return a, nil
		}
	}
	return 0, fmt.Errorf("unknown artifact %q", s)
}

// Scope decides where the artifact is scaffolded: Project = the current working
// directory; Local = the whole machine (the user's home). It's the optional 4th
// CLI argument and defaults to Project.
type Scope int

const (
	Project Scope = iota
	Local
)

var Scopes = map[Scope]string{
	Project: "project",
	Local:   "local",
}

// ParseScope resolves the CLI string (e.g. "local") into a Scope, or errors if
// the string matches no known scope.
func ParseScope(s string) (Scope, error) {
	for sc, label := range Scopes {
		if label == s {
			return sc, nil
		}
	}
	return 0, fmt.Errorf("unknown scope %q", s)
}
