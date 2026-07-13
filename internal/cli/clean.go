package cli

import (
	"agentic-developer/internal/clean"
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/doctor"
	"flag"
	"fmt"
)

// cleanCmd implements the clean command: the CLI face of the TUI's clean
// action. It scans a folder, derives the removal candidates (stale cache
// versions, orphaned caches, dead marketplaces, doctor-flagged broken
// artifacts) and lists them; --apply executes the removals.
type cleanCmd struct{}

// Name returns the command's CLI word.
func (cleanCmd) Name() string { return "clean" }

// Synopsis returns the one-line help for the command.
func (cleanCmd) Synopsis() string {
	return "list removable stale caches and broken artifacts; --apply removes them"
}

// cleanResult is one applied candidate in the report: the candidate plus
// whether its removal worked.
type cleanResult struct {
	clean.Candidate
	Removed bool   `json:"removed"`
	Error   string `json:"error,omitempty"`
}

// Run parses the optional root positional (default: the working directory)
// plus the --json and --apply flags, collects the candidates, and either
// reports them (the default dry run) or removes them.
func (c cleanCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev clean", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the report as JSON on stdout")
	apply := fs.Bool("apply", false, "execute the removals instead of only listing them")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev clean [path] [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\npath defaults to the current directory. Candidates cover stale cached")
		fmt.Fprintln(fs.Output(), "plugin versions (the installed one is kept), orphaned cache folders,")
		fmt.Fprintln(fs.Output(), "dead directory marketplaces, and artifacts the doctor flags as unloadable.")
		fmt.Fprintln(fs.Output(), "Registry entries are removed through the claude CLI, never by hand.")
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) > 1 {
		fs.Usage()
		return fmt.Errorf("unexpected extra arguments: %v", positional[1:])
	}

	root := "."
	if len(positional) == 1 {
		root = positional[0]
	}

	dirs, err := discovery.Scan(root)
	if err != nil {
		return err
	}
	candidates := clean.Collect(dirs, doctor.Check(dirs))

	if !*apply {
		if *asJSON {
			if candidates == nil {
				candidates = []clean.Candidate{} // encode as [] instead of null
			}
			return jsonOut(candidates)
		}
		printCleanReport(candidates)
		return nil
	}

	results := make([]cleanResult, 0, len(candidates))
	failed := 0
	for _, candidate := range candidates {
		result := cleanResult{Candidate: candidate, Removed: true}
		if err := clean.Apply(candidate); err != nil {
			result.Removed = false
			result.Error = err.Error()
			failed++
		}
		results = append(results, result)
	}

	if *asJSON {
		if err := jsonOut(results); err != nil {
			return err
		}
	} else {
		printCleanApplied(results)
	}
	if failed > 0 {
		return fmt.Errorf("%d removal(s) failed", failed)
	}
	return nil
}

// printCleanReport writes the human-readable dry run to stdout: one block
// per candidate and a closing summary with the reclaimable total.
func printCleanReport(candidates []clean.Candidate) {
	if len(candidates) == 0 {
		printSuccess("nothing to clean")
		return
	}

	var total int64
	for _, c := range candidates {
		printInfo("%s %s", warnMark(fmt.Sprintf("%-20s", c.Kind)), c.Reason)
		printInfo("                     %s", muted(candidateTarget(c)))
		total += c.Size
	}
	printInfo("%s", muted(fmt.Sprintf("%d candidate(s), %s reclaimable", len(candidates), clean.HumanSize(total))))
	printInfo("%s", muted("dry run: pass --apply to remove them"))
}

// printCleanApplied writes the outcome of every removal to stdout.
func printCleanApplied(results []cleanResult) {
	if len(results) == 0 {
		printSuccess("nothing to clean")
		return
	}

	var reclaimed int64
	for _, r := range results {
		if !r.Removed {
			printInfo("%s %s: %s", errorMark("✗"), candidateTarget(r.Candidate), danger(r.Error))
			continue
		}
		printSuccess("removed %s", candidateTarget(r.Candidate))
		reclaimed += r.Size
	}
	printInfo("%s", muted(fmt.Sprintf("%s reclaimed", clean.HumanSize(reclaimed))))
}

// candidateTarget phrases what a candidate acts on: the claude CLI target
// for registry entries, the folder (with its size) for disk removals.
func candidateTarget(c clean.Candidate) string {
	switch c.Action {
	case clean.ActionUninstall:
		return "plugin " + c.Arg
	case clean.ActionRemoveMarketplace:
		return "marketplace " + c.Arg
	default:
		if c.Size > 0 {
			return fmt.Sprintf("%s (%s)", c.Path, clean.HumanSize(c.Size))
		}
		return c.Path
	}
}
