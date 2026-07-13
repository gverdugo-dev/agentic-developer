package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// This file computes the stable content hash of an artifact dir, the
// primitive behind duplicate detection: two artifact dirs with the same hash
// are byte-identical copies, different hashes mean the copies drifted.

// hashSkipDirs are directory names never included in an artifact's content
// hash: VCS internals and dependency trees are not part of the artifact's
// meaningful content, and hashing them would make every scan crawl huge
// trees (a registry marketplace install is a whole git clone).
var hashSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
}

// hashArtifactDir walks the artifact dir and returns its stable content hash
// plus the per-file hashes keyed by slash-separated relative path. The dir
// hash covers the relative paths as well as the file contents, so renaming a
// file changes it as much as editing one. The walk is deterministic (the
// digest runs over sorted paths), symlinks and other irregular files are
// skipped, and a missing or unreadable dir yields "" (unhashable), never an
// error.
func hashArtifactDir(dir string) (string, map[string]string) {
	if dir == "" {
		return "", nil
	}

	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == dir {
				return err // the artifact dir itself is unreadable
			}
			return nil // an unreadable entry inside it is skipped, not fatal
		}
		if d.IsDir() {
			if path != dir && hashSkipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // symlinks and other irregular files never hash
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		sum := sha256.Sum256(raw)
		files[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		return "", nil
	}

	// The dir hash digests "path, file hash" pairs in sorted path order, so
	// it depends only on the tree's content, never on walk details.
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	digest := sha256.New()
	for _, p := range paths {
		digest.Write([]byte(p))
		digest.Write([]byte{0})
		digest.Write([]byte(files[p]))
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil)), files
}

// countFileDiffs counts the files that differ between two hashed trees: a
// path present on only one side, or present on both with different content.
// A nil side means that tree could not be hashed, so nothing can be counted.
func countFileDiffs(a, b map[string]string) int {
	if a == nil || b == nil {
		return 0
	}

	count := 0
	for path, hash := range a {
		if other, ok := b[path]; !ok || other != hash {
			count++
		}
	}
	for path := range b {
		if _, ok := a[path]; !ok {
			count++
		}
	}
	return count
}

// HashDir returns the stable content hash of an artifact dir, "" when it
// cannot be hashed. Exported so the skill installer (internal/manage) can
// verify a copy against its source with the exact hash duplicate detection
// uses: matching hashes mean the copy is byte-identical.
func HashDir(dir string) string {
	hash, _ := hashArtifactDir(dir)
	return hash
}

// ShortHash abbreviates a content hash for display: the first eight hex
// chars are plenty to tell copies apart by eye. Both the TUI and the CLI
// use it, so it lives here with the hash it abbreviates.
func ShortHash(hash string) string {
	const displayLen = 8
	if len(hash) > displayLen {
		return hash[:displayLen]
	}
	return hash
}
