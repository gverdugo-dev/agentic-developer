package cli

import (
	"agentic-developer/internal/harness"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// stubExec replaces harness.ClaudeExec for the test's lifetime, recording every
// argument list and answering with output.
func stubExec(t *testing.T, output string) *[][]string {
	t.Helper()
	var got [][]string
	orig := harness.ClaudeExec
	harness.ClaudeExec = func(args ...string) (string, error) {
		got = append(got, args)
		return output, nil
	}
	t.Cleanup(func() { harness.ClaudeExec = orig })
	return &got
}

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fnErr := fn()
	w.Close()
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if fnErr != nil {
		t.Fatalf("command failed: %v (output %q)", fnErr, out)
	}
	return string(out)
}

// TestPluginInstallJSON verifies `adev plugin install --json` delegates to
// the claude CLI with the right arguments and emits the JSON contract.
func TestPluginInstallJSON(t *testing.T) {
	got := stubExec(t, "Installed plugin ai@mkt")

	out := captureStdout(t, func() error {
		return pluginCmd{}.Run([]string{"install", "ai@mkt", "--json"})
	})

	if len(*got) != 1 || strings.Join((*got)[0], " ") != "plugin install ai@mkt" {
		t.Fatalf("claude called with %v, want plugin install ai@mkt", *got)
	}

	var report struct{ Action, Plugin, Output string }
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out)
	}
	if report.Action != "install" || report.Plugin != "ai@mkt" || report.Output != "Installed plugin ai@mkt" {
		t.Fatalf("report = %+v", report)
	}
}

// TestPluginActionsDelegate verifies every plugin action builds the right
// claude invocation.
func TestPluginActionsDelegate(t *testing.T) {
	got := stubExec(t, "")

	for _, action := range []string{"install", "enable", "disable", "uninstall"} {
		captureStdout(t, func() error {
			return pluginCmd{}.Run([]string{action, "a@m"})
		})
	}

	want := []string{
		"plugin install a@m",
		"plugin enable a@m",
		"plugin disable a@m",
		"plugin uninstall a@m",
	}
	if len(*got) != len(want) {
		t.Fatalf("got %d calls, want %d", len(*got), len(want))
	}
	for i := range want {
		if strings.Join((*got)[i], " ") != want[i] {
			t.Fatalf("call %d = %v, want %q", i, (*got)[i], want[i])
		}
	}
}

// TestPluginRejectsUnknownAction verifies an unknown action never reaches
// the claude CLI.
func TestPluginRejectsUnknownAction(t *testing.T) {
	got := stubExec(t, "")

	if err := (pluginCmd{}).Run([]string{"explode", "a@m"}); err == nil {
		t.Fatal("unknown action was accepted")
	}
	if len(*got) != 0 {
		t.Fatalf("claude was called: %v", *got)
	}
}

// TestMarketplaceAddJSON verifies `adev marketplace add --json` delegates to
// the claude CLI and emits the JSON contract.
func TestMarketplaceAddJSON(t *testing.T) {
	got := stubExec(t, "Added marketplace")

	out := captureStdout(t, func() error {
		return marketplaceCmd{}.Run([]string{"add", "owner/repo", "--json"})
	})

	if len(*got) != 1 || strings.Join((*got)[0], " ") != "plugin marketplace add owner/repo" {
		t.Fatalf("claude called with %v, want plugin marketplace add owner/repo", *got)
	}

	var report struct{ Action, Marketplace, Output string }
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out)
	}
	if report.Action != "add" || report.Marketplace != "owner/repo" || report.Output != "Added marketplace" {
		t.Fatalf("report = %+v", report)
	}
}

// TestMarketplaceRemoveDelegates verifies the remove action still builds the
// right claude invocation after the command grew flags.
func TestMarketplaceRemoveDelegates(t *testing.T) {
	got := stubExec(t, "")

	captureStdout(t, func() error {
		return marketplaceCmd{}.Run([]string{"remove", "mkt"})
	})

	if len(*got) != 1 || strings.Join((*got)[0], " ") != "plugin marketplace remove mkt" {
		t.Fatalf("claude called with %v, want plugin marketplace remove mkt", *got)
	}
}
