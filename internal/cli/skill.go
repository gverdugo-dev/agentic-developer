package cli

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/manage"
	"agentic-developer/internal/scaffolding"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// skillCmd implements the skill command: install (copy) a skill into one
// harness's skills container, or into every harness present on the machine.
// Skills are the one artifact portable across harnesses, so this is the CLI
// face of the TUI's "c" (copy to harness) on a skill row.
type skillCmd struct{}

// Name returns the command's CLI word.
func (skillCmd) Name() string { return "skill" }

// Synopsis returns the one-line help for the command.
func (skillCmd) Synopsis() string {
	return "install a skill into one harness's config dir, or into all of them"
}

// Run parses the action and source positionals plus the flags, resolves the
// source (a path or a discovered skill name) and the targets, and installs.
func (c skillCmd) Run(args []string) error {
	fs := flag.NewFlagSet("adev skill", flag.ContinueOnError)
	harnessFlag := fs.String("harness", "", "target harness: claude, codex, opencode or all (default: detect at the scope)")
	scopeFlag := fs.String("scope", "user", "target scope: user (the home config dirs) or project (the current dir's)")
	force := fs.Bool("force", false, "overwrite the skill when a target already has one with that name")
	asJSON := fs.Bool("json", false, "print the per-target results as JSON on stdout")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: adev skill install <path|discovered-name> [flags]")
		fmt.Fprintf(fs.Output(), "\n%s\n", c.Synopsis())
		fmt.Fprintln(fs.Output(), "\nThe source is a skill directory (it must carry a SKILL.md with valid")
		fmt.Fprintln(fs.Output(), "frontmatter), or the name of a skill discovered under the current dir.")
		fmt.Fprintln(fs.Output(), "--harness all installs into every harness config dir present at the scope.")
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) != 2 {
		fs.Usage()
		return fmt.Errorf("provide the action and the skill (a path or a discovered name)")
	}
	action, source := positional[0], positional[1]
	if action != "install" {
		return fmt.Errorf("unknown action %q: use install", action)
	}

	scope, err := parseSkillScope(*scopeFlag)
	if err != nil {
		return err
	}
	srcDir, err := resolveSkillSource(source)
	if err != nil {
		return err
	}

	var results []manage.SkillInstall
	if *harnessFlag == "all" {
		results, err = manage.InstallSkillAll(srcDir, scope, *force)
		if err != nil {
			return err
		}
	} else {
		h, err := resolveSkillHarness(*harnessFlag, scope)
		if err != nil {
			return err
		}
		result, err := manage.InstallSkill(srcDir, h, scope, *force)
		if err != nil {
			return err
		}
		results = []manage.SkillInstall{result}
	}

	return printSkillInstalls(filepath.Base(srcDir), results, *asJSON)
}

// parseSkillScope maps the command's scope words onto the scaffolding scopes:
// "user" (and its scaffolding synonym "local") is the home, "project" the
// current dir.
func parseSkillScope(s string) (scaffolding.Scope, error) {
	switch s {
	case "user", "local":
		return scaffolding.Local, nil
	case "project":
		return scaffolding.Project, nil
	default:
		return 0, fmt.Errorf("unknown scope %q: use user or project", s)
	}
}

// resolveSkillHarness parses an explicit harness, or detects the one whose
// config dir lives at the scope's base when none was passed.
func resolveSkillHarness(flagValue string, scope scaffolding.Scope) (scaffolding.AIHarness, error) {
	if flagValue != "" {
		return scaffolding.ParseHarness(flagValue)
	}
	base, err := scaffolding.ScopeBaseDir(scope)
	if err != nil {
		return 0, err
	}
	return scaffolding.DetectAIHarness(base)
}

// resolveSkillSource turns the positional into the skill dir to copy: an
// existing directory is used as-is; anything else is resolved as the name of
// a skill discovered under the current dir (every scan also surfaces the
// user's config dirs). A name living in more than one place is ambiguous:
// the candidates are listed and the caller reruns with one of their paths.
func resolveSkillSource(source string) (string, error) {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		return source, nil
	}

	dirs, err := discovery.Scan(".")
	if err != nil {
		return "", err
	}
	for _, g := range discovery.GroupSkills(dirs) {
		if g.Name != source {
			continue
		}
		if len(g.Locations) > 1 {
			paths := make([]string, 0, len(g.Locations))
			for _, l := range g.Locations {
				paths = append(paths, "  "+l.Item.Path)
			}
			return "", fmt.Errorf("skill %q lives in %d places:\n%s\npass the path of the copy to install", source, len(g.Locations), strings.Join(paths, "\n"))
		}
		return g.Locations[0].Item.Path, nil
	}
	return "", fmt.Errorf("%q is neither a directory nor a discovered skill name", source)
}

// printSkillInstalls reports every target's outcome, then decides the exit:
// results are always printed in full first, so a fan-out failure never hides
// the targets that did land.
func printSkillInstalls(name string, results []manage.SkillInstall, asJSON bool) error {
	if asJSON {
		type report struct {
			Skill     string                    `json:"skill"`
			Harness   string                    `json:"harness,omitempty"`
			ConfigDir string                    `json:"configDir"`
			Path      string                    `json:"path,omitempty"`
			Status    manage.SkillInstallStatus `json:"status"`
			Error     string                    `json:"error,omitempty"`
		}
		out := make([]report, 0, len(results))
		for _, r := range results {
			entry := report{Skill: name, Harness: r.Harness, ConfigDir: r.ConfigDir, Path: r.Path, Status: r.Status}
			if r.Err != nil {
				entry.Error = r.Err.Error()
			}
			out = append(out, entry)
		}
		if err := jsonOut(out); err != nil {
			return err
		}
		return skillInstallErr(results)
	}

	for _, r := range results {
		switch r.Status {
		case manage.SkillInstalled:
			printSuccess("installed %s %s", accent(name), muted("into "+r.ConfigDir))
		case manage.SkillSkipped:
			printInfo("%s", muted(fmt.Sprintf("skipped %s: %s already exists (pass --force to overwrite)", r.ConfigDir, name)))
		default:
			printInfo("%s", danger(fmt.Sprintf("failed %s: %v", r.ConfigDir, r.Err)))
		}
	}
	return skillInstallErr(results)
}

// skillInstallErr folds the per-target outcomes into the command's exit:
// any failed target errors, and a run that installed nothing (every target
// skipped an existing copy) errors too, so scripts can trust the code.
func skillInstallErr(results []manage.SkillInstall) error {
	failed, installed := 0, 0
	for _, r := range results {
		switch r.Status {
		case manage.SkillFailed:
			failed++
		case manage.SkillInstalled:
			installed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d target(s) failed", failed)
	}
	if installed == 0 {
		return fmt.Errorf("nothing installed: every target already has the skill (pass --force to overwrite)")
	}
	return nil
}
