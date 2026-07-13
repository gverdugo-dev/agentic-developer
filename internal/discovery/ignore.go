package discovery

import (
	_ "embed"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

//go:embed defaultignore
var defaultIgnoreFile string

// defaultMatcher is the embedded default ignore list, compiled once. It
// applies to the whole scan, matching paths relative to the scan root.
var defaultMatcher = gitignore.CompileIgnoreLines(
	strings.Split(strings.ReplaceAll(defaultIgnoreFile, "\r\n", "\n"), "\n")...,
)

// scopedIgnore is one .gitignore found during the walk, bound to the dir that
// contains it: its patterns only apply to paths under that base, expressed
// relative to it, which is exactly how git scopes nested ignore files.
type scopedIgnore struct {
	base    string
	matcher *gitignore.GitIgnore
}

// loadGitignore compiles dir/.gitignore, or returns nil when the file does
// not exist or cannot be parsed (a broken ignore file should not abort a
// scan, it just stops pruning).
func loadGitignore(dir string) *gitignore.GitIgnore {
	matcher, err := gitignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil
	}
	return matcher
}

// isIgnored reports whether the directory at path is excluded from the scan,
// either by the embedded default list (relative to the scan root) or by any
// .gitignore scope on the stack (relative to that scope's base). The trailing
// separator marks the path as a directory so dir-only patterns ("dist/")
// match it.
func isIgnored(root, path string, ignores []scopedIgnore) bool {
	if rel, err := filepath.Rel(root, path); err == nil {
		if defaultMatcher.MatchesPath(rel + string(filepath.Separator)) {
			return true
		}
	}

	for _, scope := range ignores {
		rel, err := filepath.Rel(scope.base, path)
		if err != nil {
			continue
		}
		if scope.matcher.MatchesPath(rel + string(filepath.Separator)) {
			return true
		}
	}

	return false
}
