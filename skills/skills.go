// Package skills bundles adev's own opinionated skills into the binary and
// installs them into a harness. These skills teach the user's agent how to use
// adev and how to build well-formed skills, plugins and plugin-marketplaces.
//
// The skill folders are embedded at build time (same self-contained philosophy
// as scaffolding's structures.json), so a single binary carries everything it
// needs to set itself up via `adev setup`.
package skills

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:adev-cli all:adev-skill-builder all:adev-plugin-builder all:adev-plugin-marketplace-builder
var bundle embed.FS

// Names returns the bundled skill names (the top-level folders), so callers can
// report what will be / was installed.
func Names() ([]string, error) {
	entries, err := bundle.ReadDir(".")
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// Install writes every bundled skill into destDir (a harness skills directory,
// e.g. ~/.claude/skills), creating it if needed. Existing files are overwritten
// so setup doubles as an update. It returns the installed skill names.
func Install(destDir string) ([]string, error) {
	err := fs.WalkDir(bundle, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, path)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := bundle.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return nil, err
	}

	return Names()
}
