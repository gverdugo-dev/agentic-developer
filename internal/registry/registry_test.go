package registry

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// testClient builds an HTTPClient pointed at one stub server for both the
// search API and the tarballs.
func testClient(server *httptest.Server) *HTTPClient {
	return &HTTPClient{
		BaseURL:        server.URL,
		TarballBaseURL: server.URL,
		HTTP:           &http.Client{Timeout: 5 * time.Second},
	}
}

// TestSearchParsesResults checks a real-shaped skills.sh response parses into
// ordered results and that the query lands escaped on the API.
func TestSearchParsesResults(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Errorf("path = %q, want /api/search", r.URL.Path)
		}
		gotQuery = r.URL.Query().Get("q")
		w.Write([]byte(`{
			"query": "find memory",
			"searchType": "fuzzy",
			"skills": [
				{"id": "acme/skills/find-skills", "skillId": "find-skills", "name": "find-skills", "installs": 540366, "source": "acme/skills"},
				{"id": "acme/skills/memory", "skillId": "memory", "name": "memory", "installs": 12, "source": "acme/skills"}
			]
		}`))
	}))
	defer server.Close()

	skills, err := testClient(server).Search("find memory")
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "find memory" {
		t.Fatalf("query on the wire = %q, want %q", gotQuery, "find memory")
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}
	first := skills[0]
	if first.SkillID != "find-skills" || first.Installs != 540366 || first.Source != "acme/skills" {
		t.Fatalf("first result = %+v", first)
	}
	if first.Ref() != "acme/skills/find-skills" {
		t.Fatalf("Ref() = %q", first.Ref())
	}
}

// TestSearchErrors checks the failure modes surface as useful errors: an
// empty query, a non-200 answer, a non-JSON body, an unreachable host.
func TestSearchErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("q") {
		case "boom":
			http.Error(w, "nope", http.StatusInternalServerError)
		default:
			w.Write([]byte("<html>not json</html>"))
		}
	}))
	defer server.Close()
	c := testClient(server)

	if _, err := c.Search("  "); err == nil {
		t.Fatal("empty query accepted")
	}
	if _, err := c.Search("boom"); err == nil {
		t.Fatal("500 answer accepted")
	}
	if _, err := c.Search("whatever"); err == nil {
		t.Fatal("non-JSON body accepted")
	}

	dead := testClient(server)
	server.Close()
	if _, err := dead.Search("query"); err == nil {
		t.Fatal("unreachable registry accepted")
	}
}

// TestNewReadsEnvOverrides checks the env vars repoint both base URLs.
func TestNewReadsEnvOverrides(t *testing.T) {
	t.Setenv(EnvBaseURL, "http://127.0.0.1:1/registry/")
	t.Setenv(EnvTarballBaseURL, "http://127.0.0.1:1/tarballs/")

	c := New()
	if c.BaseURL != "http://127.0.0.1:1/registry" {
		t.Fatalf("BaseURL = %q", c.BaseURL)
	}
	if c.TarballBaseURL != "http://127.0.0.1:1/tarballs" {
		t.Fatalf("TarballBaseURL = %q", c.TarballBaseURL)
	}
	if c.HTTP.Timeout == 0 {
		t.Fatal("the production client has no timeout")
	}
}

// TestParseRef checks the canonical reference parsing and its rejections.
func TestParseRef(t *testing.T) {
	s, err := ParseRef("vercel-labs/skills/find-skills")
	if err != nil {
		t.Fatal(err)
	}
	if s.Source != "vercel-labs/skills" || s.SkillID != "find-skills" || s.ID != "vercel-labs/skills/find-skills" {
		t.Fatalf("parsed = %+v", s)
	}

	for _, bad := range []string{
		"", "one", "one/two", "one/two/three/four",
		"one//three", "../repo/skill", "owner/./skill",
		"owner/repo/ski?ll", `owner\evil/repo/skill`,
	} {
		if _, err := ParseRef(bad); err == nil {
			t.Errorf("ParseRef(%q) accepted", bad)
		}
	}

	// The query-escaping guard also protects the tarball URL builder.
	if _, _, err := splitSource("owner/repo%2f..%2f.."); err == nil {
		t.Error("percent-encoded source accepted")
	}
	if _, err := url.Parse(DefaultBaseURL); err != nil {
		t.Errorf("default base URL invalid: %v", err)
	}
}
