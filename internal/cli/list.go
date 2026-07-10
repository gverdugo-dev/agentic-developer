package cli

import (
	"agentic-developer/internal/discovery"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// listCmd implements the list command: the CLI face of the TUI's aggregated
// views. It groups one artifact category (skills, plugins or marketplaces)
// across every config dir found under a root, showing where each one lives.
type listCmd struct{}

// Name returns the command's CLI word.
func (listCmd) Name() string { return "list" }

// Synopsis returns the one-line help for the command.
func (listCmd) Synopsis() string {
	return "list every skill, plugin or marketplace under a folder, grouped"
}

// Run parses the category positional, the optional root (default: the
// working directory) and the --json flag, then prints the grouped listing.
func (c listCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the results as JSON on stdout")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev list <skills|plugins|marketplaces> [path] [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\npath defaults to the current directory.")
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) < 1 {
		fs.Usage()
		return fmt.Errorf("provide the category: skills, plugins or marketplaces")
	}
	if len(positional) > 2 {
		fs.Usage()
		return fmt.Errorf("unexpected extra arguments: %v", positional[2:])
	}

	category := positional[0]
	root := "."
	if len(positional) == 2 {
		root = positional[1]
	}

	dirs, err := discovery.Scan(root)
	if err != nil {
		return err
	}

	switch category {
	case "skills":
		return printSkillGroups(discovery.GroupSkills(dirs), *asJSON)
	case "plugins":
		return printPluginGroups(discovery.GroupPlugins(dirs), *asJSON)
	case "marketplaces":
		return printMarketplaceGroups(discovery.GroupMarketplaces(dirs), *asJSON)
	default:
		fs.Usage()
		return fmt.Errorf("unknown category %q", category)
	}
}

// jsonOut encodes v indented to stdout.
func jsonOut(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// locationsSuffix formats the shared "in N location(s)" tail.
func locationsSuffix(n int) string {
	return muted(fmt.Sprintf("in %d location(s)", n))
}

// printSkillGroups prints every skill and where it lives.
func printSkillGroups(groups []discovery.SkillGroup, asJSON bool) error {
	if asJSON {
		type location struct {
			ConfigDir string `json:"configDir"`
			Path      string `json:"path"`
		}
		type report struct {
			Name        string     `json:"name"`
			Description string     `json:"description,omitempty"`
			Locations   []location `json:"locations"`
		}
		out := make([]report, 0, len(groups))
		for _, g := range groups {
			locs := make([]location, 0, len(g.Locations))
			for _, l := range g.Locations {
				locs = append(locs, location{ConfigDir: l.ConfigDir, Path: l.Item.Path})
			}
			out = append(out, report{Name: g.Name, Description: g.Description, Locations: locs})
		}
		return jsonOut(out)
	}

	for _, g := range groups {
		printSuccess("%s %s", accent(g.Name), locationsSuffix(len(g.Locations)))
		for _, l := range g.Locations {
			printInfo("    %s", muted(l.ConfigDir))
		}
	}
	printInfo("%s", muted(fmt.Sprintf("%d skill(s)", len(groups))))
	return nil
}

// printPluginGroups prints every plugin identity and where it is installed.
func printPluginGroups(groups []discovery.PluginGroup, asJSON bool) error {
	if asJSON {
		type location struct {
			ConfigDir string `json:"configDir"`
			Enabled   bool   `json:"enabled"`
		}
		type report struct {
			Name        string     `json:"name"`
			Marketplace string     `json:"marketplace,omitempty"`
			Version     string     `json:"version,omitempty"`
			Description string     `json:"description,omitempty"`
			Locations   []location `json:"locations"`
		}
		out := make([]report, 0, len(groups))
		for _, g := range groups {
			locs := make([]location, 0, len(g.Locations))
			for _, l := range g.Locations {
				locs = append(locs, location{ConfigDir: l.ConfigDir, Enabled: l.Item.Enabled})
			}
			out = append(out, report{
				Name: g.Name, Marketplace: g.Marketplace,
				Version: g.Version, Description: g.Description, Locations: locs,
			})
		}
		return jsonOut(out)
	}

	for _, g := range groups {
		label := accent(g.Key)
		if g.Version != "" {
			label += " " + muted(g.Version)
		}
		printSuccess("%s %s", label, locationsSuffix(len(g.Locations)))
		for _, l := range g.Locations {
			printInfo("    %s", muted(l.ConfigDir))
		}
	}
	printInfo("%s", muted(fmt.Sprintf("%d plugin(s)", len(groups))))
	return nil
}

// printMarketplaceGroups prints every marketplace and where it is
// registered.
func printMarketplaceGroups(groups []discovery.MarketplaceGroup, asJSON bool) error {
	if asJSON {
		type report struct {
			Name      string   `json:"name"`
			Source    string   `json:"source,omitempty"`
			Plugins   []string `json:"plugins,omitempty"`
			Locations []string `json:"locations"`
		}
		out := make([]report, 0, len(groups))
		for _, g := range groups {
			locs := make([]string, 0, len(g.Locations))
			for _, l := range g.Locations {
				locs = append(locs, l.ConfigDir)
			}
			out = append(out, report{Name: g.Name, Source: g.Source, Plugins: g.PluginNames, Locations: locs})
		}
		return jsonOut(out)
	}

	for _, g := range groups {
		label := accent(g.Name)
		if g.Source != "" {
			label += " " + muted("("+g.Source+")")
		}
		printSuccess("%s %s", label, locationsSuffix(len(g.Locations)))
		for _, l := range g.Locations {
			printInfo("    %s", muted(l.ConfigDir))
		}
	}
	printInfo("%s", muted(fmt.Sprintf("%d marketplace(s)", len(groups))))
	return nil
}
