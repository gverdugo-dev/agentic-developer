package cli

import (
	"agentic-developer/internal/adevfile"
	"agentic-developer/internal/discovery"
	"flag"
	"fmt"
	"os"
)

// exportCmd implements the export command: it scans the current directory
// (the same discovery the TUI and `adev scan` run) and writes an adevfile
// manifest describing that reality, so a machine's setup becomes a
// committable, syncable file.
type exportCmd struct{}

// Name returns the command's CLI word.
func (exportCmd) Name() string { return "export" }

// Synopsis returns the one-line help for the command.
func (exportCmd) Synopsis() string {
	return "write an adevfile manifest describing the discovered setup"
}

// Run parses the optional output file positional and the --json flag, scans
// the working directory, and writes the manifest: to stdout when no file is
// given (the output is the manifest JSON itself), to the file otherwise.
func (c exportCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev export", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the manifest as JSON on stdout (the default output already is)")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev export [file] [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\nThe manifest covers the user-level config dirs (user scope) and the")
		fmt.Fprintln(fs.Output(), "current directory's own config dirs (project scope). Config dirs nested")
		fmt.Fprintln(fs.Output(), "deeper belong to other projects and are left out.")
		fmt.Fprintln(fs.Output(), "\nWith no file the manifest goes to stdout; with one it is written there")
		fmt.Fprintln(fs.Output(), "(adevfile.json is the canonical name, what `adev sync` reads by default).")
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

	dirs, err := discovery.Scan(".")
	if err != nil {
		return err
	}
	manifest, skipped, err := adevfile.FromScan(dirs, ".")
	if err != nil {
		return err
	}

	// No file: the manifest itself is the stdout output, already JSON, so
	// --json changes nothing (kept for consistency with the other commands).
	if len(positional) == 0 {
		return manifest.Encode(os.Stdout)
	}

	path := positional[0]
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := manifest.Encode(file); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	if *asJSON {
		return jsonOut(struct {
			Path    string   `json:"path"`
			Skipped []string `json:"skipped,omitempty"`
		}{Path: path, Skipped: skipped})
	}

	for _, dir := range skipped {
		printInfo("%s", muted("skipped nested config dir "+dir))
	}
	printSuccess("wrote %s", accent(path))
	return nil
}
