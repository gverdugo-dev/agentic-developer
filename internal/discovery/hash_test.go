package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTree materializes files (relative path to content) under base,
// creating parent dirs as needed.
func writeTree(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// baseTree is the reference artifact every hash case mutates a copy of.
func baseTree() map[string]string {
	return map[string]string{
		"SKILL.md":            "---\nname: x\n---\n\n# Body\n",
		"references/notes.md": "notes\n",
	}
}

// TestHashArtifactDirStability verifies the content hash is stable across
// identical copies and changes exactly when the tree's content changes.
func TestHashArtifactDirStability(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(map[string]string)
		wantSame bool
	}{
		{
			name:     "identical copy in a different parent hashes the same",
			mutate:   func(map[string]string) {},
			wantSame: true,
		},
		{
			name:     "edited file content changes the hash",
			mutate:   func(f map[string]string) { f["references/notes.md"] = "edited\n" },
			wantSame: false,
		},
		{
			name: "renamed file changes the hash",
			mutate: func(f map[string]string) {
				f["references/renamed.md"] = f["references/notes.md"]
				delete(f, "references/notes.md")
			},
			wantSame: false,
		},
		{
			name:     "added file changes the hash",
			mutate:   func(f map[string]string) { f["assets/extra.txt"] = "extra\n" },
			wantSame: false,
		},
		{
			name:     "removed file changes the hash",
			mutate:   func(f map[string]string) { delete(f, "references/notes.md") },
			wantSame: false,
		},
		{
			name:     "a .git dir is not part of the hash",
			mutate:   func(f map[string]string) { f[".git/HEAD"] = "ref: refs/heads/main\n" },
			wantSame: true,
		},
		{
			name:     "a node_modules dir is not part of the hash",
			mutate:   func(f map[string]string) { f["node_modules/dep/index.js"] = "junk\n" },
			wantSame: true,
		},
	}

	reference := filepath.Join(t.TempDir(), "artifact")
	writeTree(t, reference, baseTree())
	refHash, refFiles := hashArtifactDir(reference)
	if refHash == "" {
		t.Fatal("reference dir did not hash")
	}
	if len(refFiles) != 2 {
		t.Fatalf("reference file hashes = %v, want 2 entries", refFiles)
	}
	if _, ok := refFiles["references/notes.md"]; !ok {
		t.Fatalf("file hashes miss the slash-relative path: %v", refFiles)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := baseTree()
			tc.mutate(files)

			dir := filepath.Join(t.TempDir(), "artifact")
			writeTree(t, dir, files)

			hash, _ := hashArtifactDir(dir)
			if hash == "" {
				t.Fatal("mutated dir did not hash")
			}
			if same := hash == refHash; same != tc.wantSame {
				t.Fatalf("hash equality = %v, want %v (hash %s vs reference %s)",
					same, tc.wantSame, hash, refHash)
			}
		})
	}
}

// TestHashArtifactDirUnhashable verifies the unhashable cases yield "" and
// the degenerate ones stay consistent.
func TestHashArtifactDirUnhashable(t *testing.T) {
	if hash, files := hashArtifactDir(""); hash != "" || files != nil {
		t.Fatalf("empty path: hash=%q files=%v, want unhashable", hash, files)
	}
	if hash, files := hashArtifactDir(filepath.Join(t.TempDir(), "missing")); hash != "" || files != nil {
		t.Fatalf("missing dir: hash=%q files=%v, want unhashable", hash, files)
	}

	// Two empty dirs are identical to each other.
	a, _ := hashArtifactDir(t.TempDir())
	b, _ := hashArtifactDir(t.TempDir())
	if a == "" || a != b {
		t.Fatalf("empty dirs hash to %q and %q, want equal non-empty", a, b)
	}
}

// TestCountFileDiffs verifies the drifted-file counting over hashed trees.
func TestCountFileDiffs(t *testing.T) {
	cases := []struct {
		name string
		a, b map[string]string
		want int
	}{
		{
			name: "identical trees have no diffs",
			a:    map[string]string{"x": "1", "y": "2"},
			b:    map[string]string{"x": "1", "y": "2"},
			want: 0,
		},
		{
			name: "changed content counts once",
			a:    map[string]string{"x": "1", "y": "2"},
			b:    map[string]string{"x": "1", "y": "changed"},
			want: 1,
		},
		{
			name: "a file on only one side counts on either side",
			a:    map[string]string{"x": "1", "only-a": "2"},
			b:    map[string]string{"x": "1", "only-b": "3"},
			want: 2,
		},
		{
			name: "an unhashable side counts nothing",
			a:    nil,
			b:    map[string]string{"x": "1"},
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countFileDiffs(tc.a, tc.b); got != tc.want {
				t.Fatalf("countFileDiffs = %d, want %d", got, tc.want)
			}
		})
	}
}
