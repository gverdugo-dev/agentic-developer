package cli

import (
	"agentic-developer/internal/scaffolding"
	"flag"
	"fmt"
	"path/filepath"
)

// scaffoldCmd implements the new and delete commands, which share the same
// grammar (an artifact and a name, optionally scoped and targeted at a harness)
// and differ only in whether they create or remove the artifact. The verb field
// selects which.
type scaffoldCmd struct {
	verb scaffolding.Verb
}

// Name returns the command's CLI word, taken from the verb's label.
func (c scaffoldCmd) Name() string {
	return scaffolding.Verbs[c.verb]
}

// Synopsis returns the one-line help for the command.
func (c scaffoldCmd) Synopsis() string {
	switch c.verb {
	case scaffolding.New:
		return "scaffold a new skill, plugin or plugin-marketplace"
	case scaffolding.Delete:
		return "delete a previously scaffolded artifact"
	default:
		return ""
	}
}

// Run parses the command's flags and the two positional args (artifact, name),
// then creates or removes the artifact according to the verb.
func (c scaffoldCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev "+c.Name(), flag.ContinueOnError)
	scopeStr := fs.String("scope", scaffolding.Scopes[scaffolding.Project],
		"where to place the artifact: project (current dir) or local (home)")
	harnessStr := fs.String("harness", "",
		"harness override: claude, codex or opencode (default: auto-detect)")

	// Only new can overwrite; delete has nothing to force.
	var force bool
	if c.verb == scaffolding.New {
		fs.BoolVar(&force, "force", false, "overwrite an existing artifact")
	}

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: adev %s <artifact> <name> [flags]\n\n", c.Name())
		fmt.Fprintf(fs.Output(), "%s\n\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "Artifacts: skill, plugin, plugin-marketplace")
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}

	// `adev new` with no positionals on a terminal opens an interactive form
	// instead of erroring; the flags seed its defaults. delete and any
	// non-terminal caller (a script or CI) fall through to the usage error.
	if c.verb == scaffolding.New && len(positional) == 0 && interactiveAvailable() {
		req, err := runNewForm(*scopeStr, *harnessStr, force)
		if err != nil {
			return err
		}
		return createArtifact(req)
	}

	if len(positional) < 2 {
		fs.Usage()
		return fmt.Errorf("provide the artifact and the artifact name")
	}
	if len(positional) > 2 {
		fs.Usage()
		return fmt.Errorf("unexpected extra arguments: %v", positional[2:])
	}

	req, err := newScaffoldRequest(c.verb, positional[0], positional[1], *scopeStr, *harnessStr, force)
	if err != nil {
		return err
	}

	switch c.verb {
	case scaffolding.New:
		return createArtifact(req)
	case scaffolding.Delete:
		return deleteArtifact(req)
	default:
		return fmt.Errorf("unhandled verb %v", c.verb)
	}
}

// scaffoldRequest is the parsed, validated form of a new/delete invocation. The
// raw CLI strings are turned into typed domain values here, at the boundary, so
// the rest of the flow works only with valid enums.
type scaffoldRequest struct {
	verb     scaffolding.Verb
	artifact scaffolding.Artifact
	name     string
	scope    scaffolding.Scope
	// harness overrides auto-detection; nil means "detect it from the folders".
	harness *scaffolding.AIHarness
	force   bool
}

// newScaffoldRequest validates the raw artifact/scope/harness strings and builds
// a scaffoldRequest, rejecting unknown values at the boundary.
func newScaffoldRequest(verb scaffolding.Verb, artifactStr, name, scopeStr, harnessStr string, force bool) (scaffoldRequest, error) {
	artifact, err := scaffolding.ParseArtifact(artifactStr)
	if err != nil {
		return scaffoldRequest{}, err
	}

	scope, err := scaffolding.ParseScope(scopeStr)
	if err != nil {
		return scaffoldRequest{}, err
	}

	var harness *scaffolding.AIHarness
	if harnessStr != "" {
		h, err := scaffolding.ParseHarness(harnessStr)
		if err != nil {
			return scaffoldRequest{}, err
		}
		harness = &h
	}

	return scaffoldRequest{
		verb:     verb,
		artifact: artifact,
		name:     name,
		scope:    scope,
		harness:  harness,
		force:    force,
	}, nil
}

// createArtifact scaffolds the artifact's folder structure under the dir
// resolved from the scope + harness placement, using the harness detected there.
func createArtifact(req scaffoldRequest) error {
	baseDir, harness, err := resolveTarget(req)
	if err != nil {
		return err
	}

	config := scaffolding.LoadConfig()
	resources, err := scaffolding.GetScaffoldByArtifactKey(config, harness, req.artifact)
	if err != nil {
		return err
	}

	if err := scaffolding.ApplyConfig(resources, req.name, baseDir, req.force); err != nil {
		return err
	}

	root := filepath.Join(baseDir, req.name)
	printSuccess("created %s %s at %s",
		scaffolding.Artifacts[req.artifact], accent(req.name), muted(root))
	return nil
}

// deleteArtifact removes a previously scaffolded artifact from the dir resolved
// from the scope + harness placement.
func deleteArtifact(req scaffoldRequest) error {
	baseDir, _, err := resolveTarget(req)
	if err != nil {
		return err
	}

	if err := scaffolding.RemoveConfig(req.name, baseDir); err != nil {
		return err
	}

	root := filepath.Join(baseDir, req.name)
	printSuccess("deleted %s %s at %s",
		scaffolding.Artifacts[req.artifact], accent(req.name), muted(root))
	return nil
}

// resolveTarget computes where an artifact should live: the scope base dir plus
// the harness-specific placement prefix. It also returns the detected harness,
// which createArtifact needs to look up the right scaffold.
func resolveTarget(req scaffoldRequest) (baseDir string, harness scaffolding.AIHarness, err error) {
	scopeDir, err := scaffolding.ScopeBaseDir(req.scope)
	if err != nil {
		return "", 0, err
	}

	// An explicit --harness overrides auto-detection.
	if req.harness != nil {
		harness = *req.harness
	} else {
		harness, err = scaffolding.DetectAIHarness(scopeDir)
		if err != nil {
			return "", 0, err
		}
	}

	baseDir = filepath.Join(scopeDir, scaffolding.PlacementPrefix(harness, req.artifact))
	return baseDir, harness, nil
}

// parseInterspersed parses fs allowing flags to appear before, after, or between
// the positional arguments. Go's flag package stops at the first non-flag token,
// so this re-parses the remainder after each positional, collecting the
// positionals as it goes. Unknown flags anywhere still surface as parse errors.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}
