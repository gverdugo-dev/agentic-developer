package scaffolding

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// placeholder matches any {token} inside a resource path, e.g. {skill_name}.
var placeholder = regexp.MustCompile(`\{[^}]+\}`)

//go:embed structures.json
var structuresConfig []byte

// Config is the embedded structures.json: a set of harnesses (claude, codex),
// each exposing named structures (skill, plugin, ...) -> list of resource paths.
type Config struct {
	Harness map[string]Harness `json:"harness"`
}

type Harness struct {
	Structures map[string][]string `json:"structures"`
}

func LoadConfig() Config {

	var data Config

	if err := json.Unmarshal(structuresConfig, &data); err != nil {
		slog.Error("error loading json data", "err", err)
	}

	return data
}

// ApplyConfig creates the directory/file structure described by resources,
// replacing the {placeholder} in each path with name, under baseDir.
// Paths ending in "/" are created as directories; the rest as empty files.
func ApplyConfig(resources []string, name, baseDir string) error {

	for _, resource := range resources {
		rel := placeholder.ReplaceAllString(resource, name)
		path := filepath.Join(baseDir, rel)

		if strings.HasSuffix(resource, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}

		// It's a file: ensure its parent dir exists, then create it empty.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			return err
		}
	}

	return nil
}

// RemoveConfig deletes a previously scaffolded artifact: the directory named
// after the artifact under baseDir. The whole scaffold lives under that single
// root dir, so removing it is enough. It refuses empty names or names that try
// to escape baseDir, and errors if the directory doesn't exist.
func RemoveConfig(name, baseDir string) error {

	if name == "" {
		return fmt.Errorf("artifact name is empty")
	}
	if strings.ContainsAny(name, `/\`) || name == ".." {
		return fmt.Errorf("invalid artifact name %q", name)
	}

	target := filepath.Join(baseDir, name)

	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("artifact %q not found: %w", name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", target)
	}

	return os.RemoveAll(target)
}
