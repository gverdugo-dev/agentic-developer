package cli

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/scaffolding"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// scanCmd implements the scan command: the CLI face of the same discovery
// engine the TUI uses. Every operation the TUI offers stays runnable as a
// plain command, so scripts and CI get the same capability.
type scanCmd struct{}

// Name returns the command's CLI word.
func (scanCmd) Name() string { return "scan" }

// Synopsis returns the one-line help for the command.
func (scanCmd) Synopsis() string {
	return "discover harness config dirs and their artifacts under a folder"
}

// scanReport is the JSON shape of one discovered config dir. Slices are
// initialized so empty categories encode as [] instead of null. Prompts and
// instructions only carry data for harnesses that have them (Codex prompts,
// CLAUDE.md/AGENTS.md/opencode.json awareness).
type scanReport struct {
	Path         string              `json:"path"`
	Harness      string              `json:"harness"`
	Skills       []skillReport       `json:"skills"`
	Plugins      []pluginReport      `json:"plugins"`
	Marketplaces []marketplaceReport `json:"marketplaces"`
	Prompts      []promptReport      `json:"prompts,omitempty"`
	Instructions []string            `json:"instructions,omitempty"`
}

// promptReport is the JSON shape of one prompt file.
type promptReport struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}

// skillReport is the JSON shape of one skill.
type skillReport struct {
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
	Hash        string `json:"hash,omitempty"`
}

// pluginReport is the JSON shape of one plugin. Marketplace, version and
// enabled carry data only for registry-backed plugins.
type pluginReport struct {
	Name        string `json:"name"`
	Marketplace string `json:"marketplace,omitempty"`
	Version     string `json:"version,omitempty"`
	Enabled     bool   `json:"enabled"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
	Hash        string `json:"hash,omitempty"`
}

// marketplaceReport is the JSON shape of one marketplace.
type marketplaceReport struct {
	Name    string   `json:"name"`
	Source  string   `json:"source,omitempty"`
	Path    string   `json:"path,omitempty"`
	Plugins []string `json:"plugins,omitempty"`
	Hash    string   `json:"hash,omitempty"`
}

// reportSkills, reportPlugins and reportMarketplaces map the discovery types
// to their JSON shapes.
func reportSkills(skills []discovery.Skill) []skillReport {
	out := make([]skillReport, 0, len(skills))
	for _, s := range skills {
		out = append(out, skillReport{Name: s.Name, Path: s.Path, Description: s.Description, Hash: s.Hash})
	}
	return out
}

func reportPlugins(plugins []discovery.Plugin) []pluginReport {
	out := make([]pluginReport, 0, len(plugins))
	for _, p := range plugins {
		out = append(out, pluginReport{
			Name:        p.Name,
			Marketplace: p.Marketplace,
			Version:     p.Version,
			Enabled:     p.Enabled,
			Path:        p.Path,
			Description: p.Description,
			Hash:        p.Hash,
		})
	}
	return out
}

func reportMarketplaces(marketplaces []discovery.Marketplace) []marketplaceReport {
	out := make([]marketplaceReport, 0, len(marketplaces))
	for _, mkt := range marketplaces {
		out = append(out, marketplaceReport{Name: mkt.Name, Source: mkt.Source, Path: mkt.Path, Plugins: mkt.PluginNames, Hash: mkt.Hash})
	}
	return out
}

// reportPrompts maps the prompt files to their JSON shape; nil when the
// harness has none, so the field stays omitted.
func reportPrompts(prompts []discovery.Prompt) []promptReport {
	if len(prompts) == 0 {
		return nil
	}
	out := make([]promptReport, 0, len(prompts))
	for _, p := range prompts {
		out = append(out, promptReport{Name: p.Name, Path: p.Path})
	}
	return out
}

// Run parses the optional root positional (default: the working directory)
// and the --json flag, runs the scan, and prints the findings.
func (c scanCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev scan", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the results as JSON on stdout")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev scan [path] [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\npath defaults to the current directory. The scan honors the tree's")
		fmt.Fprintln(fs.Output(), ".gitignore files plus adev's built-in ignore list.")
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

	if *asJSON {
		return printScanJSON(dirs)
	}

	printScanReport(root, dirs)
	return nil
}

// printScanJSON writes the machine-readable results to stdout.
func printScanJSON(dirs []discovery.ConfigDir) error {
	reports := make([]scanReport, 0, len(dirs))
	for _, dir := range dirs {
		reports = append(reports, scanReport{
			Path:         dir.Path,
			Harness:      scaffolding.AIHarnesses[dir.Harness],
			Skills:       reportSkills(dir.Skills),
			Plugins:      reportPlugins(dir.Plugins),
			Marketplaces: reportMarketplaces(dir.Marketplaces),
			Prompts:      reportPrompts(dir.Prompts),
			Instructions: dir.Instructions,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(reports)
}

// printScanReport writes the human-readable results to stdout.
func printScanReport(root string, dirs []discovery.ConfigDir) {
	if len(dirs) == 0 {
		printInfo("no harness config dirs found under %s", accent(root))
		return
	}

	for _, dir := range dirs {
		counts := fmt.Sprintf("(%d skills, %d plugins, %d marketplaces", len(dir.Skills), len(dir.Plugins), len(dir.Marketplaces))
		if len(dir.Prompts) > 0 {
			counts += fmt.Sprintf(", %d prompts", len(dir.Prompts))
		}
		counts += ")"
		printSuccess("%s %s %s", accent(dir.Path), scaffolding.AIHarnesses[dir.Harness], muted(counts))
	}
	printInfo("%s", muted(fmt.Sprintf("%d config dir(s) found", len(dirs))))
}
