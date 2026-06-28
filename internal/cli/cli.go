package cli

import (
	"agentic-developer/internal/scaffolding"
	"errors"
	"fmt"
	"path/filepath"
)

// ArgsBody is the parsed, validated form of a command line: verb + artifact +
// the name the user wants to give the new artifact.
type ArgsBody struct {
	Verb         scaffolding.Verb
	Artifact     scaffolding.Artifact
	ArtifactName string
	Scope        scaffolding.Scope
	// Harness is an optional override of harness auto-detection. nil means
	// "detect it from the folders".
	Harness *scaffolding.AIHarness
}

// Run is the single entry point of the CLI: it parses the raw process args and
// dispatches the resulting command. It returns an error so main() can decide
// the exit code in one place.
func Run(args []string) error {
	cmd, err := NewArgsBody(args)
	if err != nil {
		return err
	}
	return executeCommand(cmd)
}

// NewArgsBody validates the raw args and turns them into an ArgsBody. The
// strings are parsed into typed enums here, so invalid verbs/artifacts are
// rejected at the boundary.
func NewArgsBody(args []string) (ArgsBody, error) {
	if len(args) < 4 {
		return ArgsBody{}, errors.New("provide the verb, the artifact and the artifact name")
	}

	verb, err := scaffolding.ParseVerb(args[1])
	if err != nil {
		return ArgsBody{}, err
	}

	artifact, err := scaffolding.ParseArtifact(args[2])
	if err != nil {
		return ArgsBody{}, err
	}

	// Scope is the optional 4th argument; it defaults to project.
	scope := scaffolding.Project
	if len(args) >= 5 {
		scope, err = scaffolding.ParseScope(args[4])
		if err != nil {
			return ArgsBody{}, err
		}
	}

	// Harness is the optional 5th argument; when absent it is auto-detected.
	var harness *scaffolding.AIHarness
	if len(args) >= 6 {
		h, err := scaffolding.ParseHarness(args[5])
		if err != nil {
			return ArgsBody{}, err
		}
		harness = &h
	}

	return ArgsBody{
		Verb:         verb,
		Artifact:     artifact,
		ArtifactName: args[3],
		Scope:        scope,
		Harness:      harness,
	}, nil
}

// executeCommand routes the command to the right handler based on its verb.
func executeCommand(cmd ArgsBody) error {
	switch cmd.Verb {
	case scaffolding.New:
		return createArtifact(cmd)
	case scaffolding.Delete:
		return deleteArtifact(cmd)
	default:
		return fmt.Errorf("unhandled verb %v", cmd.Verb)
	}
}

// createArtifact scaffolds the artifact's folder structure under the dir
// resolved from the scope + harness placement, using the harness detected there.
func createArtifact(cmd ArgsBody) error {
	baseDir, harness, err := resolveTarget(cmd)
	if err != nil {
		return err
	}

	config := scaffolding.LoadConfig()
	resources, err := scaffolding.GetScaffoldByArtifactKey(config, harness, cmd.Artifact)
	if err != nil {
		return err
	}

	return scaffolding.ApplyConfig(resources, cmd.ArtifactName, baseDir)
}

// deleteArtifact removes a previously scaffolded artifact from the dir resolved
// from the scope + harness placement.
func deleteArtifact(cmd ArgsBody) error {
	baseDir, _, err := resolveTarget(cmd)
	if err != nil {
		return err
	}

	return scaffolding.RemoveConfig(cmd.ArtifactName, baseDir)
}

// resolveTarget computes where an artifact should live: the scope base dir plus
// the harness-specific placement prefix. It also returns the detected harness,
// which createArtifact needs to look up the right scaffold.
func resolveTarget(cmd ArgsBody) (baseDir string, harness scaffolding.AIHarness, err error) {
	scopeDir, err := scaffolding.ScopeBaseDir(cmd.Scope)
	if err != nil {
		return "", 0, err
	}

	// An explicit --harness overrides auto-detection.
	if cmd.Harness != nil {
		harness = *cmd.Harness
	} else {
		harness, err = scaffolding.DetectAIHarness(scopeDir)
		if err != nil {
			return "", 0, err
		}
	}

	baseDir = filepath.Join(scopeDir, scaffolding.PlacementPrefix(harness, cmd.Artifact))
	return baseDir, harness, nil
}
