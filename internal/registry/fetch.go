package registry

import (
	"agentic-developer/internal/discovery"
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// This file downloads a registry skill's source repo and locates the skill
// directory inside it, with stdlib only: net/http for the codeload tarball,
// archive/tar + compress/gzip for the extraction. The extraction is written
// defensively, because the tarball is third-party content:
//
//   - entries with absolute paths or any ".." traversal abort the extraction,
//   - symlinks, hardlinks and device entries are never materialized (there is
//     no link on disk a later entry could write through),
//   - the total decompressed size is capped, so a crafted archive cannot fill
//     the disk.

// maxExtractedBytes caps the total decompressed size of a fetched repo. Skill
// repos are text; 50MB is far beyond any legitimate one.
const maxExtractedBytes = 50 << 20

// FetchSkill downloads the skill's source repo as a tarball, extracts it into
// a fresh temp directory and locates the skill's directory inside it. On
// success the caller owns Fetched.Root and removes it when done; on error
// nothing is left behind.
func (c *HTTPClient) FetchSkill(s Skill) (Fetched, error) {
	owner, repo, err := splitSource(s.Source)
	if err != nil {
		return Fetched{}, err
	}
	if s.SkillID == "" {
		return Fetched{}, fmt.Errorf("the registry entry for %q has no skill id", s.Name)
	}

	endpoint := fmt.Sprintf("%s/%s/%s/tar.gz/HEAD", c.TarballBaseURL, owner, repo)
	resp, err := c.HTTP.Get(endpoint)
	if err != nil {
		return Fetched{}, fmt.Errorf("cannot download %s: %w", s.Source, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Fetched{}, fmt.Errorf("downloading %s answered %s", s.Source, resp.Status)
	}

	root, err := os.MkdirTemp("", "adev-registry-")
	if err != nil {
		return Fetched{}, err
	}
	fail := func(err error) (Fetched, error) {
		os.RemoveAll(root)
		return Fetched{}, err
	}

	if err := extractTarGz(resp.Body, root); err != nil {
		return fail(fmt.Errorf("extracting %s: %w", s.Source, err))
	}

	dir, err := locateSkillDir(root, s.SkillID)
	if err != nil {
		return fail(err)
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return fail(err)
	}

	return Fetched{Dir: dir, Root: root, Manifest: string(manifest)}, nil
}

// splitSource validates an "owner/repo" source into its two segments, with
// the same plain-name rule ParseRef applies, so a crafted source can never
// bend the tarball URL.
func splitSource(source string) (owner, repo string, err error) {
	parts := strings.Split(source, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%q is not a GitHub source: use owner/repo", source)
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `\?#%`) {
			return "", "", fmt.Errorf("%q is not a GitHub source: use owner/repo", source)
		}
	}
	return parts[0], parts[1], nil
}

// extractTarGz unpacks a gzipped tarball into dest, enforcing the defensive
// rules documented at the top of this file.
func extractTarGz(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("not a gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading the archive: %w", err)
		}

		// The path check is the load-bearing one: IsLocal rejects absolute
		// paths, any ".." traversal and empty names in one call.
		name := filepath.FromSlash(hdr.Name)
		if !filepath.IsLocal(name) {
			return fmt.Errorf("archive entry %q escapes the extraction dir", hdr.Name)
		}
		target := filepath.Join(dest, name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}

		case tar.TypeReg:
			total += hdr.Size
			if total > maxExtractedBytes {
				return fmt.Errorf("archive exceeds the %dMB extraction cap", maxExtractedBytes>>20)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// Only the exec bit survives from the archive's modes; everything
			// lands owner-writable and world-readable like a fresh checkout.
			perm := os.FileMode(0o644)
			if hdr.FileInfo().Mode().Perm()&0o100 != 0 {
				perm = 0o755
			}
			if err := writeFileFrom(tr, target, perm); err != nil {
				return err
			}

		default:
			// Symlinks, hardlinks, devices: rejected, never materialized.
			continue
		}
	}
}

// writeFileFrom streams one archive entry into target. The tar reader itself
// bounds the read at the entry's declared size, which the caller already
// counted against the extraction cap.
func writeFileFrom(r io.Reader, target string, perm os.FileMode) error {
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// locateSkillDir finds the skill's directory under the extracted root: first
// the directory whose SKILL.md frontmatter name matches skillID (read with
// the exact parsing discovery uses), then, as a fallback, a directory named
// like skillID that carries a SKILL.md. The walk is lexical, so the match is
// deterministic.
func locateSkillDir(root, skillID string) (string, error) {
	var byName, byDirName string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		manifest := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(manifest); err != nil {
			return nil
		}
		if byName == "" && discovery.ParseFrontmatter(manifest)["name"] == skillID {
			byName = path
		}
		if byDirName == "" && d.Name() == skillID {
			byDirName = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if byName != "" {
		return byName, nil
	}
	if byDirName != "" {
		return byDirName, nil
	}
	return "", fmt.Errorf("the downloaded repo has no skill %q (no SKILL.md names it and no folder is called that)", skillID)
}
