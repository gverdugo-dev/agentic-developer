package adevfile

import (
	"strings"
	"testing"
)

// TestParseValid verifies a full manifest parses into the typed model.
func TestParseValid(t *testing.T) {
	manifest := `{
	  "version": 1,
	  "harnesses": {
	    "claude": {
	      "user": {
	        "skills": ["dataviz", "adev-cli"],
	        "plugins": ["personal@gonzaloverdugo", "bare-folder-plugin"],
	        "marketplaces": [{"name": "gonzaloverdugo", "source": "owner/repo"}]
	      },
	      "project": {"skills": ["local-skill"]}
	    },
	    "codex": {"user": {"skills": ["dataviz"]}}
	  }
	}`

	f, err := Parse(strings.NewReader(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	claude := f.Harnesses["claude"]
	if claude.User == nil || claude.Project == nil {
		t.Fatalf("claude scopes not parsed: %+v", claude)
	}
	if got := strings.Join(claude.User.Skills, ","); got != "dataviz,adev-cli" {
		t.Fatalf("claude user skills = %q", got)
	}
	if got := strings.Join(claude.User.Plugins, ","); got != "personal@gonzaloverdugo,bare-folder-plugin" {
		t.Fatalf("claude user plugins = %q", got)
	}
	if len(claude.User.Marketplaces) != 1 || claude.User.Marketplaces[0].Source != "owner/repo" {
		t.Fatalf("claude user marketplaces = %+v", claude.User.Marketplaces)
	}
	if claude.Project.Skills[0] != "local-skill" {
		t.Fatalf("claude project skills = %v", claude.Project.Skills)
	}
	if codex := f.Harnesses["codex"]; codex.User == nil || codex.User.Skills[0] != "dataviz" {
		t.Fatalf("codex user = %+v", codex.User)
	}
}

// TestParseRejects verifies every malformed manifest fails with an error
// naming the problem spot.
func TestParseRejects(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		wantIn   string
	}{
		{
			name:     "wrong version",
			manifest: `{"version": 2, "harnesses": {}}`,
			wantIn:   "version must be 1",
		},
		{
			name:     "missing version",
			manifest: `{"harnesses": {}}`,
			wantIn:   "version must be 1",
		},
		{
			name:     "unknown harness",
			manifest: `{"version": 1, "harnesses": {"cursor": {}}}`,
			wantIn:   "harnesses.cursor: unknown harness",
		},
		{
			name:     "unknown field typo",
			manifest: `{"version": 1, "harnesses": {"claude": {"user": {"skils": ["x"]}}}}`,
			wantIn:   "skils",
		},
		{
			name:     "unknown scope",
			manifest: `{"version": 1, "harnesses": {"claude": {"global": {}}}}`,
			wantIn:   "global",
		},
		{
			name:     "empty skill name",
			manifest: `{"version": 1, "harnesses": {"claude": {"user": {"skills": [" "]}}}}`,
			wantIn:   "harnesses.claude.user.skills[0]",
		},
		{
			name:     "skill with path",
			manifest: `{"version": 1, "harnesses": {"claude": {"user": {"skills": ["a/b"]}}}}`,
			wantIn:   "harnesses.claude.user.skills[0]",
		},
		{
			name:     "plugin key without name",
			manifest: `{"version": 1, "harnesses": {"claude": {"user": {"plugins": ["@mkt"]}}}}`,
			wantIn:   "harnesses.claude.user.plugins[0]",
		},
		{
			name:     "plugin key without marketplace",
			manifest: `{"version": 1, "harnesses": {"claude": {"user": {"plugins": ["name@"]}}}}`,
			wantIn:   "harnesses.claude.user.plugins[0]",
		},
		{
			name:     "plugin key with two ats",
			manifest: `{"version": 1, "harnesses": {"claude": {"user": {"plugins": ["a@b@c"]}}}}`,
			wantIn:   "harnesses.claude.user.plugins[0]",
		},
		{
			name:     "marketplace without name",
			manifest: `{"version": 1, "harnesses": {"claude": {"user": {"marketplaces": [{"source": "owner/repo"}]}}}}`,
			wantIn:   "harnesses.claude.user.marketplaces[0]",
		},
		{
			name:     "not json",
			manifest: `skills: [a]`,
			wantIn:   "not a valid adevfile",
		},
		{
			name:     "trailing data",
			manifest: `{"version": 1, "harnesses": {}} {"again": true}`,
			wantIn:   "trailing data",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.manifest))
			if err == nil {
				t.Fatalf("manifest was accepted: %s", tc.manifest)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not mention %q", err, tc.wantIn)
			}
		})
	}
}

// TestEncodeParseRoundTrip verifies Encode writes exactly what Parse reads.
func TestEncodeParseRoundTrip(t *testing.T) {
	original := File{
		Version: CurrentVersion,
		Harnesses: map[string]Harness{
			"claude": {
				User: &Entry{
					Skills:       []string{"dataviz"},
					Plugins:      []string{"personal@gonzaloverdugo"},
					Marketplaces: []Marketplace{{Name: "gonzaloverdugo", Source: "owner/repo"}},
				},
			},
			"opencode": {Project: &Entry{Plugins: []string{"notify"}}},
		},
	}

	var buf strings.Builder
	if err := original.Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	parsed, err := Parse(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("Parse of encoded manifest: %v\n%s", err, buf.String())
	}

	var reencoded strings.Builder
	if err := parsed.Encode(&reencoded); err != nil {
		t.Fatalf("re-Encode: %v", err)
	}
	if buf.String() != reencoded.String() {
		t.Fatalf("round trip drifted:\n%s\nvs\n%s", buf.String(), reencoded.String())
	}
}
