// Package registry talks to public skill registries. The first (and for now
// only) implementation is skills.sh, the public directory and leaderboard of
// agent skills: searching it returns skills indexed from public GitHub repos,
// and fetching one downloads that repo as a codeload tarball, so adev stays
// self-contained (no git, no npx).
//
// The client hides behind the Client interface so other registries can plug
// in later, and both base URLs are overridable (options and env vars), so
// tests and the e2e suite never touch the network.
//
// Security stance: a third-party skill is a prompt the user's agent will
// execute. This package only searches, downloads and locates; it never
// installs. The callers (CLI and TUI) must show the fetched SKILL.md and get
// an explicit confirmation before handing the directory to the installer.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// The production endpoints and their override env vars. ADEV_REGISTRY_URL
// points the search API somewhere else (a stub in tests); the tarball base is
// separate because the real one is a different host (codeload.github.com).
const (
	DefaultBaseURL        = "https://www.skills.sh"
	DefaultTarballBaseURL = "https://codeload.github.com"
	EnvBaseURL            = "ADEV_REGISTRY_URL"
	EnvTarballBaseURL     = "ADEV_REGISTRY_TARBALL_URL"
)

// requestTimeout bounds every registry request. It is generous because the
// tarball download shares the client, and a source repo can be a few MB.
const requestTimeout = 60 * time.Second

// maxSearchResponseBytes caps how much of a search response is read: a
// legitimate result list is a few KB, so 2MB means something is wrong.
const maxSearchResponseBytes = 2 << 20

// Skill is one skill of the registry, as the search API returns it.
type Skill struct {
	// ID is the full identity, "owner/repo/skill-id".
	ID string `json:"id"`
	// SkillID is the skill's own identifier inside its source repo.
	SkillID string `json:"skillId"`
	// Name is the display name.
	Name string `json:"name"`
	// Installs is the registry's install count across agents.
	Installs int `json:"installs"`
	// Source is the GitHub repo the skill lives in, "owner/repo".
	Source string `json:"source"`
}

// Ref is the skill's canonical reference, "owner/repo/skill-id": what
// `adev skill install <ref> --from-registry` takes.
func (s Skill) Ref() string {
	if s.ID != "" {
		return s.ID
	}
	return s.Source + "/" + s.SkillID
}

// Fetched is a skill downloaded to disk, ready to preview and (only after an
// explicit confirmation) install.
type Fetched struct {
	// Dir is the located skill directory, inside Root.
	Dir string
	// Root is the temp extraction root; the caller removes it when done.
	Root string
	// Manifest is the content of the skill's SKILL.md, for the preview.
	Manifest string
}

// Client is what adev needs from a skill registry: find skills, and fetch
// one's source to disk. Implementations must never install anything.
type Client interface {
	Search(query string) ([]Skill, error)
	FetchSkill(s Skill) (Fetched, error)
}

// HTTPClient is the skills.sh implementation of Client.
type HTTPClient struct {
	// BaseURL is the search API host (no trailing slash).
	BaseURL string
	// TarballBaseURL is the codeload-style host serving repo tarballs at
	// /<owner>/<repo>/tar.gz/HEAD (no trailing slash).
	TarballBaseURL string
	// HTTP performs the requests; it must carry a timeout.
	HTTP *http.Client
}

// New builds the production client: the skills.sh endpoints, overridden by
// the env vars when set, with a bounded request timeout.
func New() *HTTPClient {
	c := &HTTPClient{
		BaseURL:        DefaultBaseURL,
		TarballBaseURL: DefaultTarballBaseURL,
		HTTP:           &http.Client{Timeout: requestTimeout},
	}
	if v := os.Getenv(EnvBaseURL); v != "" {
		c.BaseURL = strings.TrimRight(v, "/")
	}
	if v := os.Getenv(EnvTarballBaseURL); v != "" {
		c.TarballBaseURL = strings.TrimRight(v, "/")
	}
	return c
}

// Search queries the registry and returns its results in the order the API
// ranks them (the callers keep that order).
func (c *HTTPClient) Search(query string) ([]Skill, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, errors.New("the search query is empty")
	}

	endpoint := c.BaseURL + "/api/search?q=" + url.QueryEscape(q)
	resp, err := c.HTTP.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("registry unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry answered %s to the search", resp.Status)
	}

	var payload struct {
		Skills []Skill `json:"skills"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxSearchResponseBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("registry answered invalid JSON: %w", err)
	}
	return payload.Skills, nil
}

// ParseRef parses a canonical skill reference, "owner/repo/skill-id", into
// the Skill a fetch needs. Every segment must be a plain name: an empty one,
// a dot segment or anything path-like is rejected before it can reach a URL
// or a filesystem path.
func ParseRef(ref string) (Skill, error) {
	parts := strings.Split(ref, "/")
	if len(parts) != 3 {
		return Skill{}, fmt.Errorf("%q is not a registry skill reference: use owner/repo/skill-id", ref)
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `\?#%`) {
			return Skill{}, fmt.Errorf("%q is not a registry skill reference: use owner/repo/skill-id", ref)
		}
	}
	return Skill{
		ID:      ref,
		SkillID: parts[2],
		Name:    parts[2],
		Source:  parts[0] + "/" + parts[1],
	}, nil
}
