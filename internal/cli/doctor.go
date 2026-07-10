package cli

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/doctor"
	"flag"
	"fmt"
)

// doctorCmd implements the doctor command: the CLI face of the TUI's doctor
// view. It scans a folder for harness config dirs, runs every health check
// over what it finds, and prints the findings as a prioritized report.
type doctorCmd struct{}

// Name returns the command's CLI word.
func (doctorCmd) Name() string { return "doctor" }

// Synopsis returns the one-line help for the command.
func (doctorCmd) Synopsis() string {
	return "check every artifact under a folder and report what is broken"
}

// Run parses the optional root positional (default: the working directory)
// and the --json flag, runs the checks, and prints the findings.
func (c doctorCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev doctor", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the findings as JSON on stdout")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev doctor [path] [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\npath defaults to the current directory. Checks cover skills (SKILL.md,")
		fmt.Fprintln(fs.Output(), "frontmatter, the 200-line house cap), plugin and marketplace manifests,")
		fmt.Fprintln(fs.Output(), "and Claude's plugin registry (ghost plugins, cache drift, dead sources).")
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
	findings := doctor.Check(dirs)

	if *asJSON {
		if findings == nil {
			findings = []doctor.Finding{} // encode as [] instead of null
		}
		return jsonOut(findings)
	}

	printDoctorReport(findings)
	return nil
}

// printDoctorReport writes the human-readable findings to stdout: one block
// per finding, errors first (Check already sorted them), and a closing
// summary line.
func printDoctorReport(findings []doctor.Finding) {
	errors, warnings := 0, 0
	for _, f := range findings {
		mark := errorMark("✗ error  ")
		if f.Severity == doctor.Warning {
			mark = warnMark("! warning")
			warnings++
		} else {
			errors++
		}
		printInfo("%s %s", mark, f.Message)
		printInfo("          %s", muted(f.Path))
		if f.FixHint != "" {
			printInfo("          %s %s", muted("fix:"), f.FixHint)
		}
	}

	if len(findings) == 0 {
		printSuccess("no problems found")
		return
	}
	printInfo("%s", muted(fmt.Sprintf("%d error(s), %d warning(s)", errors, warnings)))
}
