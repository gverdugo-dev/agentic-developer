package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// updateRepo is the GitHub repository adev updates itself from.
const updateRepo = "gverdugo-dev/agentic-developer"

// runUpdate replaces the running adev binary with the latest release build for
// this OS/arch. It downloads the matching asset from GitHub Releases, verifies
// its checksum, and atomically swaps the executable in place, with no Go
// toolchain or external tools required.
func runUpdate() error {
	client := &http.Client{Timeout: 60 * time.Second}

	latest, err := latestTag(client)
	if err != nil {
		return fmt.Errorf("checking the latest version: %w", err)
	}

	current := resolveVersion()
	if current == latest {
		fmt.Printf("adev is already up to date (%s)\n", current)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating the adev binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	asset, isZip := assetName()
	base := fmt.Sprintf("https://github.com/%s/releases/latest/download", updateRepo)

	fmt.Printf("adev: updating %s -> %s ...\n", current, latest)

	archive, err := download(client, base+"/"+asset)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", asset, err)
	}
	if err := verifyChecksum(client, base+"/checksums.txt", asset, archive); err != nil {
		return err
	}

	binData, err := extractBinary(archive, isZip)
	if err != nil {
		return fmt.Errorf("extracting the binary: %w", err)
	}
	if err := replaceExecutable(exe, binData); err != nil {
		return fmt.Errorf("replacing the adev binary at %q: %w", exe, err)
	}

	fmt.Printf("adev: updated to %s\n", latest)
	return nil
}

// latestTag returns the tag name of the repository's latest GitHub Release.
func latestTag(c *http.Client) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", updateRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("no release tag found")
	}
	return rel.TagName, nil
}

// assetName returns the release asset for the current platform and whether it is
// a zip (Windows) rather than a tar.gz.
func assetName() (name string, isZip bool) {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("adev_windows_%s.zip", runtime.GOARCH), true
	}
	return fmt.Sprintf("adev_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH), false
}

func download(c *http.Client, url string) ([]byte, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// verifyChecksum confirms data matches its sha256 in the checksums file.
func verifyChecksum(c *http.Client, url, asset string, data []byte) error {
	sums, err := download(c, url)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}

	want := ""
	for line := range strings.SplitSeq(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum listed for %s", asset)
	}

	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s, aborting", asset)
	}
	return nil
}

// extractBinary pulls the adev binary out of a downloaded archive.
func extractBinary(archive []byte, isZip bool) ([]byte, error) {
	if isZip {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == "adev.exe" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("adev.exe not found in archive")
	}

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == "adev" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("adev not found in archive")
}

// replaceExecutable atomically swaps the binary at path with newData. It writes
// to a temp file in the same directory (so the rename stays on one filesystem)
// and renames it over the original. On Windows the running file can't be
// overwritten, so it is moved aside first.
func replaceExecutable(path string, newData []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".adev-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // harmless no-op once the rename succeeds

	if _, err := tmp.Write(newData); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		old := path + ".old"
		os.Remove(old)
		if err := os.Rename(path, old); err != nil {
			return err
		}
		if err := os.Rename(tmpName, path); err != nil {
			os.Rename(old, path) // try to restore the original
			return err
		}
		os.Remove(old) // best effort; may stay until the process exits
		return nil
	}

	return os.Rename(tmpName, path)
}
