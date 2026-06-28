package scaffolding

// !TODO
func DetectAIHarness() AIHarness {

	return 0

}

// GetScaffoldByArtifactKey returns the list of resource paths for an artifact.
// The Artifact is already validated (it came from ParseArtifact), so the JSON
// key lookup is guaranteed to hit.
func GetScaffoldByArtifactKey(config Config, artifact Artifact) []string {
	key := artifactJSONKey[artifact]
	return config.Harness["claude"].Structures[key]
}
