package manage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDeleteArtifactValidation verifies the safety rules: only absolute
// paths that are a config dir or live inside one can be deleted.
func TestDeleteArtifactValidation(t *testing.T) {
	base := t.TempDir()

	// A folder outside any config dir is refused.
	outside := filepath.Join(base, "just-a-folder")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DeleteArtifact(outside); err == nil {
		t.Fatal("deleted a folder outside a config dir")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("the refused folder was still removed")
	}

	// A relative path is refused.
	if err := DeleteArtifact("relative/path"); err == nil {
		t.Fatal("accepted a relative path")
	}

	// A skill inside a config dir is deleted.
	skill := filepath.Join(base, ".claude", "skills", "my-skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DeleteArtifact(skill); err != nil {
		t.Fatalf("refused a valid artifact: %v", err)
	}
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatal("the artifact still exists")
	}

	// A whole config dir is deletable too.
	claude := filepath.Join(base, ".claude")
	if err := DeleteArtifact(claude); err != nil {
		t.Fatalf("refused a config dir: %v", err)
	}
	if _, err := os.Stat(claude); !os.IsNotExist(err) {
		t.Fatal("the config dir still exists")
	}
}

// TestClaudeOperationsBuildTheRightCommands verifies the claude CLI
// delegation uses the exact argument shapes, via a stubbed executor.
func TestClaudeOperationsBuildTheRightCommands(t *testing.T) {
	var got [][]string
	orig := Exec
	Exec = func(args ...string) (string, error) {
		got = append(got, args)
		return "ok", nil
	}
	defer func() { Exec = orig }()

	if _, err := SetPluginEnabled("a@m", true); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPluginEnabled("a@m", false); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPlugin("a@m"); err != nil {
		t.Fatal(err)
	}
	if _, err := UninstallPlugin("a@m"); err != nil {
		t.Fatal(err)
	}
	if _, err := AddMarketplace("owner/repo"); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveMarketplace("mkt"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"plugin", "enable", "a@m"},
		{"plugin", "disable", "a@m"},
		{"plugin", "install", "a@m"},
		{"plugin", "uninstall", "a@m"},
		{"plugin", "marketplace", "add", "owner/repo"},
		{"plugin", "marketplace", "remove", "mkt"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d calls, want %d", len(got), len(want))
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("call %d = %v, want %v", i, got[i], want[i])
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("call %d = %v, want %v", i, got[i], want[i])
			}
		}
	}
}
