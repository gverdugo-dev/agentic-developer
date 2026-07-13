// Package manage executes the mutations adev offers on discovered
// resources: deleting artifacts (or whole config dirs) from the filesystem,
// and plugin/marketplace operations delegated to the claude CLI, which owns
// that registry and its cache. Both the TUI and the CLI commands run through
// this package, so every operation stays available in both.
package manage

import (
	"agentic-developer/internal/scaffolding"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DeleteArtifact removes the directory at path, after validating it is safe
// to touch: an absolute, existing directory that either is a harness config
// dir or lives inside one. Anything else is refused, so adev can never be
// talked into deleting an arbitrary folder.
func DeleteArtifact(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute")
	}
	clean := filepath.Clean(path)

	info, err := os.Lstat(clean)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", clean)
	}
	if !insideConfigDir(clean) {
		return fmt.Errorf("refusing to delete %q: not a harness config dir nor inside one", clean)
	}

	return os.RemoveAll(clean)
}

// insideConfigDir reports whether some component of path is a harness config
// dir marker (.claude, .codex, .opencode), which makes path the config dir
// itself or something inside one.
func insideConfigDir(path string) bool {
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		if _, ok := scaffolding.HarnessForMarker(component); ok {
			return true
		}
	}
	return false
}
