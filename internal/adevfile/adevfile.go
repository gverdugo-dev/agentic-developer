// Package adevfile defines the adevfile manifest: a declarative description
// of the agent setup a machine or project should have, per harness and
// scope: which skills, which plugins (name@marketplace) and which
// marketplaces (with the source they are added from). `adev export` writes
// one from the discovered reality, and `adev sync` diffs one against that
// reality and applies what is missing.
//
// The format is JSON and the canonical file name is adevfile.json. JSON on
// purpose: it is already the house format (structures.json, plugin.json,
// marketplace.json and Claude's own registry files are all JSON), the
// standard library parses it, and pulling in a YAML dependency for one
// small hand-editable file would break adev's self-contained rule.
//
// The smallest useful manifest:
//
//	{
//	  "version": 1,
//	  "harnesses": {
//	    "claude": {
//	      "user": {
//	        "skills": ["dataviz"],
//	        "plugins": ["personal@gonzaloverdugo"],
//	        "marketplaces": [{"name": "gonzaloverdugo", "source": "owner/repo"}]
//	      }
//	    }
//	  }
//	}
//
// A declared scope means "sync manages this config dir": everything listed
// must exist there, and anything found there but not listed is reported as
// extra. A scope that is not declared is left completely alone.
package adevfile

import (
	"agentic-developer/internal/scaffolding"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// CurrentVersion is the manifest schema version this build reads and writes.
const CurrentVersion = 1

// The scope labels of a manifest entry. "user" is the harness config dir in
// the user's home (~/.claude); "project" is the one at the root sync or
// export runs in (./.claude).
const (
	ScopeUser    = "user"
	ScopeProject = "project"
)

// Scopes lists the scope labels in their canonical order.
var Scopes = []string{ScopeUser, ScopeProject}

// File is one parsed adevfile manifest.
type File struct {
	// Version is the schema version; must equal CurrentVersion.
	Version int `json:"version"`
	// Harnesses maps each harness label ("claude", "codex", "opencode") to
	// its declared scopes.
	Harnesses map[string]Harness `json:"harnesses"`
}

// Harness declares what one harness should have, per scope. A nil scope is
// undeclared: sync does not manage that config dir at all.
type Harness struct {
	User    *Entry `json:"user,omitempty"`
	Project *Entry `json:"project,omitempty"`
}

// Entry returns the declaration of one scope label, nil when undeclared.
func (h Harness) Entry(scope string) *Entry {
	switch scope {
	case ScopeUser:
		return h.User
	case ScopeProject:
		return h.Project
	default:
		return nil
	}
}

// Entry declares the artifacts one harness config dir should hold.
type Entry struct {
	// Skills lists skill folder names.
	Skills []string `json:"skills,omitempty"`
	// Plugins lists plugin keys: "name@marketplace" for registry plugins,
	// a bare name for folder plugins.
	Plugins []string `json:"plugins,omitempty"`
	// Marketplaces lists the marketplaces that must be registered.
	Marketplaces []Marketplace `json:"marketplaces,omitempty"`
}

// Marketplace declares one plugin marketplace: its registry name and the
// source it is added from (whatever the harness CLI accepts: a GitHub
// "owner/repo", a git URL, a local path). An empty source still diffs by
// name, but sync cannot add it and reports it as manual.
type Marketplace struct {
	Name   string `json:"name"`
	Source string `json:"source,omitempty"`
}

// Parse reads and validates a manifest. Unknown fields are rejected, so a
// typo ("skils") surfaces as a parse error instead of a silently ignored
// declaration.
func Parse(r io.Reader) (File, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var f File
	if err := dec.Decode(&f); err != nil {
		return File{}, fmt.Errorf("not a valid adevfile: %w", err)
	}
	if dec.More() {
		return File{}, fmt.Errorf("not a valid adevfile: trailing data after the manifest object")
	}
	if err := f.Validate(); err != nil {
		return File{}, err
	}
	return f, nil
}

// ParseFile parses the manifest at path, prefixing errors with the path.
func ParseFile(path string) (File, error) {
	file, err := os.Open(path)
	if err != nil {
		return File{}, err
	}
	defer file.Close()

	f, err := Parse(file)
	if err != nil {
		return File{}, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// Encode writes the manifest as indented JSON, the same shape Parse reads.
func (f File) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(f)
}

// Validate checks the manifest's schema: the version, that every harness
// label is a known harness, and that every declared item is well formed.
// Errors name the exact spot ("harnesses.claude.user.plugins[0]") so a hand
// edit is easy to fix.
func (f File) Validate() error {
	if f.Version != CurrentVersion {
		return fmt.Errorf("version must be %d, got %d", CurrentVersion, f.Version)
	}

	labels := make([]string, 0, len(f.Harnesses))
	for label := range f.Harnesses {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	for _, label := range labels {
		if _, err := scaffolding.ParseHarness(label); err != nil {
			return fmt.Errorf("harnesses.%s: unknown harness, use claude, codex or opencode", label)
		}
		for _, scope := range Scopes {
			entry := f.Harnesses[label].Entry(scope)
			if entry == nil {
				continue
			}
			if err := validateEntry(fmt.Sprintf("harnesses.%s.%s", label, scope), *entry); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateEntry checks one scope declaration's items.
func validateEntry(prefix string, e Entry) error {
	for i, name := range e.Skills {
		if strings.TrimSpace(name) == "" || strings.ContainsRune(name, '/') {
			return fmt.Errorf("%s.skills[%d]: a skill is a plain folder name, got %q", prefix, i, name)
		}
	}
	for i, key := range e.Plugins {
		if !validPluginKey(key) {
			return fmt.Errorf("%s.plugins[%d]: a plugin key looks like \"name\" or \"name@marketplace\", got %q", prefix, i, key)
		}
	}
	for i, mkt := range e.Marketplaces {
		if strings.TrimSpace(mkt.Name) == "" {
			return fmt.Errorf("%s.marketplaces[%d]: a marketplace needs a name", prefix, i)
		}
	}
	return nil
}

// validPluginKey reports whether key is "name" or "name@marketplace" with
// both halves non-empty.
func validPluginKey(key string) bool {
	if strings.TrimSpace(key) == "" || strings.Count(key, "@") > 1 {
		return false
	}
	name, marketplace, found := strings.Cut(key, "@")
	if strings.TrimSpace(name) == "" {
		return false
	}
	return !found || strings.TrimSpace(marketplace) != ""
}
