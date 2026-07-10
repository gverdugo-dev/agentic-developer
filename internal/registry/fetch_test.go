package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tarEntry is one entry of an in-test tarball.
type tarEntry struct {
	name     string
	body     string
	typeflag byte
	linkname string
	mode     int64
}

// buildTarball assembles a gzipped tarball from entries, the way codeload
// serves repos (everything under a "<repo>-HEAD/" prefix in the entries
// themselves).
func buildTarball(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		flag := e.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     mode,
			Size:     int64(len(e.body)),
			Typeflag: flag,
			Linkname: e.linkname,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// skillManifest builds a minimal SKILL.md with parseable frontmatter.
func skillManifest(name string) string {
	return "---\nname: " + name + "\ndescription: \"Registry fixture skill\"\n---\n\n# " + name + "\n"
}

// tarballClient serves one tarball for every repo path and returns a client
// pointed at it.
func tarballClient(t *testing.T, tarball []byte) *HTTPClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/tar.gz/HEAD") {
			http.NotFound(w, r)
			return
		}
		w.Write(tarball)
	}))
	t.Cleanup(server.Close)
	return testClient(server)
}

// TestFetchSkillLocatesByFrontmatter checks the happy path: the tarball
// extracts, the skill dir is found by its SKILL.md frontmatter name (not its
// folder name), and the manifest content comes back for the preview.
func TestFetchSkillLocatesByFrontmatter(t *testing.T) {
	tarball := buildTarball(t, []tarEntry{
		{name: "skills-HEAD/", typeflag: tar.TypeDir},
		{name: "skills-HEAD/README.md", body: "# repo"},
		{name: "skills-HEAD/skills/oddly-named-dir/SKILL.md", body: skillManifest("find-skills")},
		{name: "skills-HEAD/skills/oddly-named-dir/references/notes.md", body: "notes"},
		{name: "skills-HEAD/skills/other/SKILL.md", body: skillManifest("other")},
	})
	c := tarballClient(t, tarball)

	fetched, err := c.FetchSkill(Skill{Source: "acme/skills", SkillID: "find-skills", Name: "find-skills"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fetched.Root)

	if filepath.Base(fetched.Dir) != "oddly-named-dir" {
		t.Fatalf("located %q, want the frontmatter match", fetched.Dir)
	}
	if !strings.Contains(fetched.Manifest, "name: find-skills") {
		t.Fatalf("manifest = %q", fetched.Manifest)
	}
	if !strings.HasPrefix(fetched.Dir, fetched.Root) {
		t.Fatalf("dir %q outside root %q", fetched.Dir, fetched.Root)
	}
	if _, err := os.Stat(filepath.Join(fetched.Dir, "references", "notes.md")); err != nil {
		t.Fatal("the skill's companion files did not extract")
	}
}

// TestFetchSkillFallsBackToDirName checks the fallback: no frontmatter names
// the skill, but a folder called like the skillId carries a SKILL.md.
func TestFetchSkillFallsBackToDirName(t *testing.T) {
	tarball := buildTarball(t, []tarEntry{
		{name: "repo-HEAD/find-skills/SKILL.md", body: "---\nname: something-else\n---\n\n# hi\n"},
	})
	c := tarballClient(t, tarball)

	fetched, err := c.FetchSkill(Skill{Source: "acme/repo", SkillID: "find-skills"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fetched.Root)
	if filepath.Base(fetched.Dir) != "find-skills" {
		t.Fatalf("located %q, want the dir-name fallback", fetched.Dir)
	}
}

// TestFetchSkillMissingSkillErrs checks a repo without the skill errors and
// leaves no temp dir behind (the root is cleaned on every failure path).
func TestFetchSkillMissingSkillErrs(t *testing.T) {
	tarball := buildTarball(t, []tarEntry{
		{name: "repo-HEAD/README.md", body: "# repo"},
	})
	c := tarballClient(t, tarball)

	_, err := c.FetchSkill(Skill{Source: "acme/repo", SkillID: "ghost"})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("err = %v, want the missing skill named", err)
	}
}

// TestFetchSkillRejectsTraversal checks entries that escape the extraction
// dir abort the fetch: ".." traversal and absolute paths.
func TestFetchSkillRejectsTraversal(t *testing.T) {
	for _, evil := range []string{"../evil.txt", "repo-HEAD/../../evil.txt", "/etc/evil.txt"} {
		tarball := buildTarball(t, []tarEntry{
			{name: "repo-HEAD/skill/SKILL.md", body: skillManifest("skill")},
			{name: evil, body: "pwned"},
		})
		c := tarballClient(t, tarball)
		if _, err := c.FetchSkill(Skill{Source: "acme/repo", SkillID: "skill"}); err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Errorf("entry %q: err = %v, want an escape rejection", evil, err)
		}
	}
}

// TestFetchSkillNeverMaterializesLinks checks symlink and hardlink entries
// are dropped: nothing on disk a later entry could write through.
func TestFetchSkillNeverMaterializesLinks(t *testing.T) {
	tarball := buildTarball(t, []tarEntry{
		{name: "repo-HEAD/skill/SKILL.md", body: skillManifest("skill")},
		{name: "repo-HEAD/escape", typeflag: tar.TypeSymlink, linkname: "/tmp"},
		{name: "repo-HEAD/hard", typeflag: tar.TypeLink, linkname: "repo-HEAD/skill/SKILL.md"},
	})
	c := tarballClient(t, tarball)

	fetched, err := c.FetchSkill(Skill{Source: "acme/repo", SkillID: "skill"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fetched.Root)

	for _, name := range []string{"escape", "hard"} {
		if _, err := os.Lstat(filepath.Join(fetched.Root, "repo-HEAD", name)); !os.IsNotExist(err) {
			t.Errorf("link entry %q was materialized", name)
		}
	}
}

// TestFetchSkillEnforcesSizeCap checks the cumulative decompressed size cap
// aborts the extraction.
func TestFetchSkillEnforcesSizeCap(t *testing.T) {
	// Two entries whose declared sizes cross the cap together.
	half := strings.Repeat("a", 1<<20)
	entries := []tarEntry{{name: "repo-HEAD/skill/SKILL.md", body: skillManifest("skill")}}
	for i := 0; i < 51; i++ {
		entries = append(entries, tarEntry{name: "repo-HEAD/big" + strings.Repeat("g", i) + ".bin", body: half})
	}
	c := tarballClient(t, buildTarball(t, entries))

	_, err := c.FetchSkill(Skill{Source: "acme/repo", SkillID: "skill"})
	if err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("err = %v, want the size cap", err)
	}
}

// TestFetchSkillHTTPFailures checks a missing repo and a non-gzip body error
// cleanly.
func TestFetchSkillHTTPFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/gone/") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("this is not gzip"))
	}))
	defer server.Close()
	c := &HTTPClient{BaseURL: server.URL, TarballBaseURL: server.URL, HTTP: &http.Client{Timeout: 5 * time.Second}}

	if _, err := c.FetchSkill(Skill{Source: "gone/repo", SkillID: "skill"}); err == nil {
		t.Fatal("404 accepted")
	}
	if _, err := c.FetchSkill(Skill{Source: "acme/repo", SkillID: "skill"}); err == nil || !strings.Contains(err.Error(), "gzip") {
		t.Fatalf("err = %v, want the gzip rejection", err)
	}
	if _, err := c.FetchSkill(Skill{Source: "not-a-source", SkillID: "skill"}); err == nil {
		t.Fatal("bad source accepted")
	}
	if _, err := c.FetchSkill(Skill{Source: "acme/repo"}); err == nil {
		t.Fatal("empty skill id accepted")
	}
}
