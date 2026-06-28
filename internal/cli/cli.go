package cli

import (
	"agentic-developer/internal/scaffolding"
	"errors"
	"fmt"
)

// ArgsBody is the parsed, validated form of a command line: verb + artifact +
// the name the user wants to give the new artifact.
type ArgsBody struct {
	Verb         scaffolding.Verb
	Artifact     scaffolding.Artifact
	ArtifactName string
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

	return ArgsBody{
		Verb:         verb,
		Artifact:     artifact,
		ArtifactName: args[3],
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

// createArtifact scaffolds the artifact's folder structure in the current dir.
func createArtifact(cmd ArgsBody) error {
	config := scaffolding.LoadConfig()
	resources := scaffolding.GetScaffoldByArtifactKey(config, cmd.Artifact)
	return scaffolding.ApplyConfig(resources, cmd.ArtifactName, ".")
}

// deleteArtifact removes a previously scaffolded artifact.
func deleteArtifact(cmd ArgsBody) error {
	return fmt.Errorf("delete not implemented yet")
}
