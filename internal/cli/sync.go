package cli

import (
	"agentic-developer/internal/adevfile"
	"agentic-developer/internal/discovery"
	"flag"
	"fmt"
)

// syncCmd implements the sync command: it diffs an adevfile manifest against
// the discovered reality and prints the plan (the default dry run), or
// executes the installable missing items with --apply. Extras are reported,
// never deleted: there is no prune in this version.
type syncCmd struct{}

// Name returns the command's CLI word.
func (syncCmd) Name() string { return "sync" }

// Synopsis returns the one-line help for the command.
func (syncCmd) Synopsis() string {
	return "diff an adevfile manifest against reality; --apply installs what is missing"
}

// defaultManifest is the manifest file sync reads when none is given: the
// canonical adevfile name, in the directory sync runs in.
const defaultManifest = "adevfile.json"

// syncReport is the JSON shape of a dry run: the verdict plus every item.
type syncReport struct {
	InSync  bool            `json:"inSync"`
	Missing int             `json:"missing"`
	Extra   int             `json:"extra"`
	Items   []adevfile.Item `json:"items"`
}

// syncResult is one applied item in the --apply report.
type syncResult struct {
	adevfile.Item
	Applied bool   `json:"applied"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Run parses the optional manifest positional (default: adevfile.json) plus
// the --json and --apply flags, diffs manifest against the reality scanned
// from the working directory, and reports or applies the plan.
//
// The exit code is script-friendly, diff-style: 0 when reality satisfies the
// manifest (or --apply completed every installable item), non-zero when
// items are missing on a dry run or an apply failed. Extras never fail the
// command.
func (c syncCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev sync", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the plan (or the apply results) as JSON on stdout")
	apply := fs.Bool("apply", false, "execute the installable missing items instead of only listing them")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev sync [file] [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\nfile defaults to adevfile.json in the current directory. The manifest's")
		fmt.Fprintln(fs.Output(), "user scope is the harness config dir in your home, the project scope the")
		fmt.Fprintln(fs.Output(), "one here. Missing plugins and marketplaces are installed through the")
		fmt.Fprintln(fs.Output(), "harness's own CLI; missing skills are reported for manual install until")
		fmt.Fprintln(fs.Output(), "adev grows the cross-harness skill installer. Extra artifacts (present")
		fmt.Fprintln(fs.Output(), "but undeclared) are reported and never deleted.")
		fmt.Fprintln(fs.Output(), "\nExit code: 0 when in sync, 1 when items are missing or an apply failed.")
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

	path := defaultManifest
	if len(positional) == 1 {
		path = positional[0]
	}

	manifest, err := adevfile.ParseFile(path)
	if err != nil {
		return err
	}
	dirs, err := discovery.Scan(".")
	if err != nil {
		return err
	}
	plan, err := adevfile.Diff(manifest, dirs, ".")
	if err != nil {
		return err
	}

	if !*apply {
		return c.report(plan, *asJSON)
	}
	return c.apply(plan, *asJSON)
}

// report prints the dry-run plan and returns the diff-style verdict.
func (syncCmd) report(plan adevfile.Plan, asJSON bool) error {
	if asJSON {
		items := plan.Items
		if items == nil {
			items = []adevfile.Item{} // encode as [] instead of null
		}
		if err := jsonOut(syncReport{
			InSync:  plan.InSync(),
			Missing: plan.Missing(),
			Extra:   plan.Extra(),
			Items:   items,
		}); err != nil {
			return err
		}
	} else {
		printPlan(plan)
	}

	if !plan.InSync() {
		return fmt.Errorf("%d item(s) missing", plan.Missing())
	}
	return nil
}

// apply executes every installable missing item and reports each outcome.
// Manual missing items are reported as skipped but do not fail the run:
// there is nothing adev can execute for them.
func (syncCmd) apply(plan adevfile.Plan, asJSON bool) error {
	var results []syncResult
	failed := 0
	for _, item := range plan.Items {
		if item.State != adevfile.StateMissing || item.Action == adevfile.ActionManual {
			continue
		}
		result := syncResult{Item: item, Applied: true}
		output, err := adevfile.Apply(item)
		result.Output = output
		if err != nil {
			result.Applied = false
			result.Error = err.Error()
			failed++
		}
		results = append(results, result)
	}

	if asJSON {
		if results == nil {
			results = []syncResult{}
		}
		if err := jsonOut(results); err != nil {
			return err
		}
	} else {
		printApplied(plan, results)
	}

	if failed > 0 {
		return fmt.Errorf("%d apply step(s) failed", failed)
	}
	return nil
}

// printPlan writes the human-readable dry run: the missing and extra items
// (satisfied ones are only counted) and the verdict.
func printPlan(plan adevfile.Plan) {
	if len(plan.Items) == 0 {
		printInfo("the manifest declares nothing to check")
		return
	}

	for _, item := range plan.Items {
		switch item.State {
		case adevfile.StateMissing:
			printInfo("%s %s %s", warnMark("missing"), item.Label(), muted("("+item.Action+")"))
		case adevfile.StateExtra:
			printInfo("%s   %s %s", muted("extra"), item.Label(), muted("(not in the manifest)"))
		}
	}

	satisfied := len(plan.Items) - plan.Missing() - plan.Extra()
	printInfo("%s", muted(fmt.Sprintf("%d satisfied, %d missing, %d extra", satisfied, plan.Missing(), plan.Extra())))
	if plan.InSync() {
		printSuccess("in sync")
	} else {
		printInfo("%s", muted("dry run: pass --apply to install what is missing"))
	}
}

// printApplied writes the outcome of every executed item, plus the manual
// leftovers the user still has to handle.
func printApplied(plan adevfile.Plan, results []syncResult) {
	for _, r := range results {
		if !r.Applied {
			printInfo("%s %s: %s", errorMark("✗"), r.Label(), danger(r.Error))
			continue
		}
		if r.Output != "" {
			printInfo("%s", muted(r.Output))
		}
		printSuccess("%s %s", pastTense(r.Action), accent(r.Label()))
	}

	manual := 0
	for _, item := range plan.Items {
		if item.State == adevfile.StateMissing && item.Action == adevfile.ActionManual {
			printInfo("%s  %s %s", warnMark("manual"), item.Label(), muted("(install it yourself)"))
			manual++
		}
	}
	if len(results) == 0 && manual == 0 {
		printSuccess("nothing to apply: already in sync")
	}
}

// pastTense phrases an executed action for the success line.
func pastTense(action string) string {
	if action == adevfile.ActionAdd {
		return "added"
	}
	return "installed"
}
