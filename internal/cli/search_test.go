package cli

import (
	"agentic-developer/internal/registry"
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubRegistry starts an httptest server that answers the search API with
// one fixture skill and serves its source repo as a tarball, then points the
// registry client env overrides at it. Nothing touches the network.
func stubRegistry(t *testing.T) {
	t.Helper()

	manifest := "---\nname: demo-skill\ndescription: \"Registry fixture skill\"\n---\n\n# demo-skill\n"
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "demo-HEAD/skills/demo-skill/SKILL.md", Mode: 0o644, Size: int64(len(manifest))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	tarball := buf.Bytes()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/search":
			w.Write([]byte(`{"skills": [{"id": "acme/demo/demo-skill", "skillId": "demo-skill", "name": "demo-skill", "installs": 4242, "source": "acme/demo"}]}`))
		case strings.HasSuffix(r.URL.Path, "/tar.gz/HEAD"):
			w.Write(tarball)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	t.Setenv(registry.EnvBaseURL, server.URL)
	t.Setenv(registry.EnvTarballBaseURL, server.URL)
}

// TestSearchJSON drives `adev search --json` against the stub registry and
// checks the results come back as the registry ranked them.
func TestSearchJSON(t *testing.T) {
	stubRegistry(t)

	out := captureStdout(t, func() error {
		return searchCmd{}.Run([]string{"demo", "skill", "--json"})
	})

	var skills []registry.Skill
	if err := json.Unmarshal([]byte(out), &skills); err != nil {
		t.Fatalf("bad JSON %q: %v", out, err)
	}
	if len(skills) != 1 || skills[0].SkillID != "demo-skill" || skills[0].Installs != 4242 {
		t.Fatalf("skills = %+v", skills)
	}
}

// TestSearchRequiresQuery checks the usage error.
func TestSearchRequiresQuery(t *testing.T) {
	if err := (searchCmd{}).Run([]string{"--json"}); err == nil {
		t.Fatal("empty query accepted")
	}
}

// TestRegistryInstallRefusesWithoutYes checks the security gate: with no
// terminal attached (tests never have one) a registry install without --yes
// is refused, in both text and JSON mode, and nothing lands on disk.
func TestRegistryInstallRefusesWithoutYes(t *testing.T) {
	stubRegistry(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := captureStdoutRun(t, func() error {
		return skillCmd{}.Run([]string{"install", "acme/demo/demo-skill", "--from-registry", "--harness", "claude"})
	})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("text mode: err = %v, want the --yes refusal", err)
	}

	err = captureStdoutRun(t, func() error {
		return skillCmd{}.Run([]string{"install", "acme/demo/demo-skill", "--from-registry", "--harness", "claude", "--json"})
	})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("json mode: err = %v, want the --yes refusal", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "demo-skill")); !os.IsNotExist(err) {
		t.Fatal("a refused install still landed on disk")
	}
}

// TestRegistryInstallWithYes checks the approved path: the skill is fetched,
// installed through the T7 copier, and the JSON report carries the SKILL.md
// that was installed.
func TestRegistryInstallWithYes(t *testing.T) {
	stubRegistry(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() error {
		return skillCmd{}.Run([]string{"install", "acme/demo/demo-skill", "--from-registry", "--harness", "claude", "--yes", "--json"})
	})

	var reports []struct {
		Skill   string `json:"skill"`
		Status  string `json:"status"`
		Path    string `json:"path"`
		SkillMd string `json:"skillMd"`
	}
	if err := json.Unmarshal([]byte(out), &reports); err != nil {
		t.Fatalf("bad JSON %q: %v", out, err)
	}
	if len(reports) != 1 || reports[0].Status != "installed" {
		t.Fatalf("reports = %+v", reports)
	}
	if !strings.Contains(reports[0].SkillMd, "name: demo-skill") {
		t.Fatalf("report misses the installed SKILL.md: %+v", reports[0])
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "demo-skill", "SKILL.md")); err != nil {
		t.Fatal("the skill did not land in ~/.claude")
	}
}

// TestRegistryInstallBadRef checks a malformed reference is rejected before
// anything is fetched.
func TestRegistryInstallBadRef(t *testing.T) {
	stubRegistry(t)
	if err := (skillCmd{}).Run([]string{"install", "not-a-ref", "--from-registry", "--yes"}); err == nil {
		t.Fatal("bad reference accepted")
	}
}

// captureStdoutRun runs fn with stdout swallowed (the preview prints there)
// and returns fn's error.
func captureStdoutRun(t *testing.T, fn func() error) error {
	t.Helper()
	var err error
	captureStdout(t, func() error {
		err = fn()
		return nil
	})
	return err
}
