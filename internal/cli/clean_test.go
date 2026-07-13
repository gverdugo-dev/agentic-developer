package cli

import (
	"agentic-developer/internal/clean"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// cleanFixture builds a scan root holding a .claude config dir with an
// empty plugin registry and one orphaned cache folder, and isolates HOME so
// discovery never prepends the developer's real config dirs (and --apply
// can never touch them).
func cleanFixture(t *testing.T) (root, orphan string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	root = t.TempDir()
	configDir := filepath.Join(root, ".claude")
	orphan = filepath.Join(configDir, "plugins", "cache", "mkt", "orphan")
	if err := os.MkdirAll(filepath.Join(orphan, "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(orphan, "1.0.0", "plugin.json"):                 `{"name": "orphan"}`,
		filepath.Join(configDir, "plugins", "installed_plugins.json"): `{"version": 2, "plugins": {}}`,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, orphan
}

// TestCleanDryRunJSON verifies the default run only reports: the JSON lists
// the orphan candidate and nothing is deleted.
func TestCleanDryRunJSON(t *testing.T) {
	got := stubExec(t, "")
	root, orphan := cleanFixture(t)

	out := captureStdout(t, func() error {
		return cleanCmd{}.Run([]string{root, "--json"})
	})

	var candidates []clean.Candidate
	if err := json.Unmarshal([]byte(out), &candidates); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out)
	}
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates %+v, want 1", len(candidates), candidates)
	}
	c := candidates[0]
	if c.Kind != clean.KindCacheOrphan || c.Path != orphan || c.Size <= 0 {
		t.Fatalf("candidate = %+v, want the orphan cache dir with a size", c)
	}

	if _, err := os.Stat(orphan); err != nil {
		t.Fatal("dry run deleted the orphan")
	}
	if len(*got) != 0 {
		t.Fatalf("dry run called the claude CLI: %v", *got)
	}
}

// TestCleanApply verifies --apply removes the orphan from disk and reports
// it, without ever calling the claude CLI for a plain folder.
func TestCleanApply(t *testing.T) {
	got := stubExec(t, "")
	root, orphan := cleanFixture(t)

	out := captureStdout(t, func() error {
		return cleanCmd{}.Run([]string{root, "--apply", "--json"})
	})

	var results []struct {
		clean.Candidate
		Removed bool   `json:"removed"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out)
	}
	if len(results) != 1 || !results[0].Removed || results[0].Error != "" {
		t.Fatalf("results = %+v, want one successful removal", results)
	}

	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("the orphan still exists after --apply")
	}
	if len(*got) != 0 {
		t.Fatalf("a disk-only removal called the claude CLI: %v", *got)
	}
}

// TestCleanNothingToClean verifies a healthy tree reports an empty JSON
// array, not null.
func TestCleanNothingToClean(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() error {
		return cleanCmd{}.Run([]string{root, "--json"})
	})
	if string(out) != "[]\n" {
		t.Fatalf("output = %q, want an empty JSON array", out)
	}
}
