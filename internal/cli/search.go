package cli

import (
	"agentic-developer/internal/registry"
	"flag"
	"fmt"
)

// newRegistryClient builds the registry client commands talk to. It is a
// variable so tests swap in a stub; the production client reads the
// ADEV_REGISTRY_URL / ADEV_REGISTRY_TARBALL_URL overrides itself.
var newRegistryClient = func() registry.Client { return registry.New() }

// searchCmd implements the search command: query the skills.sh registry and
// list the matching skills. It only searches; installing a result goes
// through `adev skill install <owner/repo/skill-id> --from-registry`, which
// previews the SKILL.md and asks for confirmation.
type searchCmd struct{}

// Name returns the command's CLI word.
func (searchCmd) Name() string { return "search" }

// Synopsis returns the one-line help for the command.
func (searchCmd) Synopsis() string {
	return "search the skills.sh registry for skills"
}

// Run parses the query and the --json flag, searches, and prints the results
// in the order the registry ranks them.
func (c searchCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev search", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the results as JSON on stdout")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev search <query> [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\nEach result is a skill reference (owner/repo/skill-id); install one with")
		fmt.Fprintln(fs.Output(), "adev skill install <reference> --from-registry.")
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) == 0 {
		fs.Usage()
		return fmt.Errorf("provide a search query")
	}

	// A multi-word query works unquoted: the positionals are joined.
	query := ""
	for i, p := range positional {
		if i > 0 {
			query += " "
		}
		query += p
	}

	skills, err := newRegistryClient().Search(query)
	if err != nil {
		return err
	}

	if *asJSON {
		if skills == nil {
			skills = []registry.Skill{}
		}
		return jsonOut(skills)
	}

	if len(skills) == 0 {
		printInfo("%s", muted("no skills match "+fmt.Sprintf("%q", query)))
		return nil
	}
	for _, s := range skills {
		printInfo("%s %s %s", accent(s.Name), muted(s.Source), muted(fmt.Sprintf("%d installs", s.Installs)))
	}
	printInfo("%s", muted("install one: adev skill install <owner/repo/skill-id> --from-registry"))
	return nil
}
