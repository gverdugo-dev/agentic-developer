package cli

import (
	"agentic-developer/internal/scaffolding"
	"errors"
)

type ArgsBody struct {
	Verb         scaffolding.Verb
	Artifact     scaffolding.Artifact
	ArtifactName string
}

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

func executeCommand(args ArgsBody) {

}
