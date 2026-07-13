package harness

import (
	"agentic-developer/internal/scaffolding"
	"os"
	"path/filepath"
	"testing"
)

// write creates file under dir (creating parents) with content.
func write(t *testing.T, dir, file, content string) string {
	t.Helper()
	path := filepath.Join(dir, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAdapterRegistryOrder verifies All returns one adapter per harness in
// the canonical detection priority order (Claude, Codex, opencode), matching
// the scaffolding domain's single source of truth.
func TestAdapterRegistryOrder(t *testing.T) {
	all := All()
	order := scaffolding.HarnessesInOrder()
	if len(all) != len(order) {
		t.Fatalf("got %d adapters, want %d", len(all), len(order))
	}
	for i, id := range order {
		if all[i].ID() != id {
			t.Fatalf("adapter %d is %v, want %v", i, all[i].ID(), id)
		}
		if all[i].Marker() != scaffolding.MarkerFor(id) {
			t.Fatalf("adapter %d marker = %q, want %q", i, all[i].Marker(), scaffolding.MarkerFor(id))
		}
	}
}

// TestAdapterLookups verifies ForID and ForMarker resolve every adapter and
// reject unknown inputs.
func TestAdapterLookups(t *testing.T) {
	for _, ad := range All() {
		byID, ok := ForID(ad.ID())
		if !ok || byID.ID() != ad.ID() {
			t.Fatalf("ForID(%v) = %v, %v", ad.ID(), byID, ok)
		}
		byMarker, ok := ForMarker(ad.Marker())
		if !ok || byMarker.ID() != ad.ID() {
			t.Fatalf("ForMarker(%q) = %v, %v", ad.Marker(), byMarker, ok)
		}
	}

	if _, ok := ForMarker(".unknown"); ok {
		t.Fatal("ForMarker accepted an unknown marker")
	}
}

// TestInstructionFilesLookInAndNextToConfigDir verifies the shared
// instruction-file lookup: the file counts both inside the config dir (the
// user-level copy) and next to it (the project-level copy).
func TestInstructionFilesLookInAndNextToConfigDir(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	codex, _ := ForID(scaffolding.Codex)
	if got := codex.InstructionFiles(configDir); got != nil {
		t.Fatalf("no files on disk, got %v", got)
	}

	inside := write(t, configDir, "AGENTS.md", "# global\n")
	sibling := write(t, base, "AGENTS.md", "# project\n")
	got := codex.InstructionFiles(configDir)
	if len(got) != 2 || got[0] != inside || got[1] != sibling {
		t.Fatalf("instruction files = %v, want [%s %s]", got, inside, sibling)
	}
}
