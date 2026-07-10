# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this is

`adev` (module `agentic-developer`) is a small Go CLI that scaffolds, discovers
and manages AI coding-agent artifacts (skills, plugins, plugin-marketplaces)
for a detected or explicit harness (Claude Code, Codex, opencode). Run bare on
a terminal it opens a lazygit-style TUI over the same discovery and management
engine the commands use. It is self-contained: the folder layouts and its own
bundled skills are embedded in the binary at build time, so a single binary
carries everything it needs.

The opinionated approach to building skills and plugins lives in
[PHILOSOPHY.md](PHILOSOPHY.md); read it before touching the bundled skills.

## Commands

```bash
make build       # build ./bin/adev (version injected via -ldflags)
make run         # build, then run
make install     # go install ./cmd/adev
make release     # cross-compile all targets into dist/ (the release workflow runs this)
make e2e         # pty e2e suite for the TUI (requires `expect`); E2E=<name> runs one test
go build ./...   # compile
go test ./...    # unit tests (cli, clean, discovery, doctor, harness, manage, tui)
go vet ./...     # static checks
gofmt -l .       # formatting check (empty output = clean)
go doc ./internal/scaffolding   # browse package docs
```

Before opening a PR, the whole battery must pass: `go build ./...`,
`go vet ./...`, `gofmt -l .` (empty output), `go test ./...`, `make build`,
plus `make e2e` whenever the TUI changed. For `setup` changes also verify
against a temporary HOME (see "Verifying setup" below).

## Architecture

Three layers, raw args flow down through them:

```
cmd/adev/main.go            Entry point: wires the program, owns the exit code.
internal/cli/               Boundary: every invocation is a Command that parses
                            its own flags and runs itself; Run dispatches (and
                            opens the TUI when called bare on a terminal).
  cli.go                    Command interface, the command registry, Run dispatch, help.
  output.go                 Styled user-facing output (lipgloss); degrades to plain text.
  interactive.go            huh form for `adev new` with no args on a terminal.
  scaffold_cmd.go           scaffoldCmd (new + delete): flag parsing + apply/remove.
  scan.go                   scanCmd: discovery scan of a folder tree (--json).
  list.go                   listCmd: grouped skills/plugins/marketplaces (--json, --duplicates).
  doctor.go                 doctorCmd: the health report over internal/doctor (--json).
  clean.go                  cleanCmd: removal candidates via internal/clean (--json, --apply).
  rm.go                     rmCmd: guarded delete of an artifact or config dir.
  plugin.go                 pluginCmd: install/enable/disable/uninstall via the claude CLI.
  marketplace.go            marketplaceCmd: marketplace add/remove via the claude CLI.
  setup.go                  setupCmd: install bundled skills into a harness.
  version.go                versionCmd + the ldflags-injected Version var.
  update.go                 updateCmd: self-update from the latest release.
internal/scaffolding/       Scaffolding domain core.
  types.go                  Typed model (AIHarness, Verb, Artifact, Scope) + Parse* validators.
  scaffold.go               Config load + ApplyConfig/RemoveConfig engine.
  utils.go                  Harness detection, placement, lookups.
  structures.json           Embedded folder layouts (//go:embed).
internal/harness/           One adapter per AI harness (see "How harness adapters work").
  harness.go                Adapter interface, shared types, the adapter registry.
  claude.go                 Claude adapter: registry reader, manifests, claude CLI executor.
  codex.go                  Codex adapter: skills + prompts, AGENTS.md awareness.
  opencode.go               opencode adapter: skills + TS plugins, opencode.json awareness.
internal/discovery/         Discovery domain core (shared by CLI and TUI).
  discovery.go              Scan: the gitignore-aware walk; collection driven by adapters.
  ignore.go                 Default ignore list + scoped .gitignore matching.
  meta.go                   SKILL.md frontmatter parsing (the standard every harness shares).
  compat.go                 Deprecated ReadClaudeRegistry shim over the Claude adapter.
  hash.go                   Content hashes per artifact dir, behind drift detection.
  aggregate.go              Cross-path grouping (SkillGroup, PluginGroup, ...) + DriftState.
internal/doctor/            Health checks: finding model + skill checks + adapter validators.
internal/clean/             Removal candidates (stale caches, orphans, dead marketplaces,
                            broken artifacts) built on discovery + doctor; Apply removes one.
internal/manage/            Mutations on discovered resources.
  manage.go                 DeleteArtifact: guarded filesystem deletes.
  claude.go                 Plugin/marketplace helpers forwarding to the Claude adapter.
internal/tui/               The lazygit-style dashboard (Bubble Tea).
  tui.go                    Root model: state machine (intro -> dashboard), global keys.
  intro.go                  The logo decode animation.
  dashboard.go              Views, navigation, root input, confirm + actions.
  previews.go               Preview/detail content for every selectable item.
  styles.go                 Lipgloss styles on the brand palette.
internal/brand/             The shared color palette.
skills/                     Bundled skills, embedded via //go:embed.
  skills.go                 Embed + Install into a harness skills dir.
  adev-cli/, adev-skill-builder/, adev-plugin-builder/, adev-plugin-marketplace-builder/
e2e/                        pty e2e suite for the TUI (expect); see "How the e2e suite works".
```

### How the CLI is organized

The `cli` package follows the subcommand pattern used by the `go` tool itself:
every invocation is a `Command`, and a registry maps the first arg to it.

```go
type Command interface {
	Name() string     // the word typed on the command line
	Synopsis() string // one-line description for the top-level help
	Run(args []string) error // parses the args after the name, then runs
}
```

`Run` (in `cli.go`) looks the command up in the `commands` map and hands it
everything after the command name; each command then owns the parsing of its own
flags via a `flag.FlagSet`. This keeps a single model: `new`, `delete`, `setup`,
`update`, and `version` are all sibling commands, instead of special-casing the
flagless ones outside an arg parser.

- `new` and `delete` are one type, `scaffoldCmd`, parameterized by a `Verb`: they
  share the same grammar (`<artifact> <name>` plus `--scope`, `--harness`, and
  `--force` for `new` only) and differ only in apply vs remove. The two
  positionals are validated into typed domain values (`scaffoldRequest`) at the
  boundary, so the rest of the flow works only with valid enums.
- Flags may appear before, after, or between the positionals. Go's `flag` package
  stops at the first non-flag token, so `parseInterspersed` re-parses the
  remainder after each positional. Unknown flags still surface as errors.
- Adding a command is implementing `Command` and adding it to `commands` (and to
  `order`, which fixes the help listing order); the dispatcher does not change.
- `adev new` with no positionals, run on a terminal, opens an interactive `huh`
  form (`interactive.go`) that collects the same fields; passed flags seed its
  defaults, and the collected values go through the same `newScaffoldRequest`
  validation. `interactiveAvailable` gates it on stdin and stdout both being
  TTYs, so a non-terminal caller (a script or CI) keeps the old usage error
  instead of blocking on a prompt. `delete` is never interactive.

### How scaffolding works

`structures.json` maps each harness to the per-artifact file/dir layout. The
destination of `adev new <artifact> <name>` is built as:

```
<scope base dir> / .<harness> / <artifact subdir> / <name>
```

Scope `project` resolves to the current dir, `local` to the user's home. Harness
detection looks only in that base dir for a marker (`.claude`, `.codex`,
`.opencode`), with priority Claude, Codex, opencode, and errors if none is found
and none is passed. `new` refuses to overwrite an existing artifact unless
`--force`.

### How setup works

`adev setup [harness]` installs adev's own bundled skills. It always targets the
user's config (home), never the project, writing to `~/.<harness>/skills/`. It
overwrites adev's own skill files (so re-running is an update) but never deletes
the skills directory or touches other skills. There is no pruning of files
removed from the bundle.

### How harness adapters work

`internal/harness` makes every harness a first-class citizen behind one
`Adapter` interface: the marker dir that identifies it, which artifact kinds
live in which containers, its instruction files, a registry reader (where the
harness has one), manifest validators, and an operation executor (where a CLI
owns the registry). The adapter registry (`All`, `ForID`, `ForMarker`)
preserves the detection priority (claude, codex, opencode). Discovery, the
doctor and the TUI consume adapters and never branch on a concrete harness,
so supporting a new harness (gemini-cli, cursor) is implementing `Adapter`
and appending it to the list.

Adapters are honest about capability: Claude is the deep one (registry,
enabled state, cache, claude CLI operations via the stubbable
`harness.ClaudeExec`); Codex has skills, a prompts folder and AGENTS.md;
opencode has skills, TypeScript plugins (package.json metadata, index.ts
entry check) and opencode.json. Neither of the last two fakes a registry or
operations it does not have.

### How discovery works

`discovery.Scan(root)` walks the tree under root looking for harness config
dirs (`.claude`, `.codex`, `.opencode`) and summarizes what lives in each,
collecting exactly the containers the dir's adapter declares: skills (with
SKILL.md frontmatter), plugins, marketplaces, prompt files and instruction
docs. Rules:

- The walk respects the same boundaries git does: an embedded default ignore
  list applies everywhere, and each `.gitignore` applies to its own subtree.
  Symlinks are never followed; unreadable dirs are skipped, not fatal.
- A config dir is a leaf: it is collected and never descended into.
- The user's home config dirs are always prepended to the results, whatever
  the root, because user-level config applies to every project.
- A harness with its own registry (Claude: `plugins/installed_plugins.json`,
  `plugins/known_marketplaces.json`, `settings.json` enabledPlugins) prefers
  it, the same data its `/plugins` screen shows; a missing or unparsable
  registry falls back to the folder layout.

`aggregate.go` regroups the per-dir results into the artifact-centric views
(one group per skill name / plugin identity / marketplace name, each with its
locations), which power the TUI's views 2-4 and `adev list`. Each artifact dir
gets a stable content hash (`hash.go`: relative paths + file contents, walked
deterministically), so a group knows its `DriftState`: copies identical
everywhere, drifted (with a per-location differing-file count), single, or
unknown. `adev list --duplicates` filters to multi-location groups.

### How management works

`internal/manage` executes every mutation, for both the TUI and the CLI:

- `DeleteArtifact(path)` removes a directory only after validating it is
  absolute, exists, and is a harness config dir or inside one; anything else
  is refused, so adev can never be talked into deleting an arbitrary folder.
- Plugin and marketplace operations (install/enable/disable/uninstall,
  marketplace add/remove) forward to the Claude adapter's executor, which
  shells out to the `claude` CLI: it owns the registry and its cache. Never
  write those JSON files by hand. Tests stub `harness.ClaudeExec`.

`internal/doctor` is the read-only health layer: `doctor.Check(dirs)` runs
the shared skill checks (missing/invalid SKILL.md or frontmatter, the
200-line house cap) plus each adapter's validators (invalid manifests,
enabled-but-not-installed plugins, cache/registry drift, dead marketplace
directory sources, TS plugins without their entry point) and returns
prioritized findings with fix hints. It powers `adev doctor` and the TUI's
doctor view.

`internal/clean` is the actuator on top: `clean.Collect(dirs, findings)`
derives removal candidates (stale cached plugin versions keeping the
installed one, orphaned cache folders, dead directory marketplaces, and the
doctor findings whose remedy is removal), each with its reclaimable size and
its removal action. `clean.Apply` executes one candidate: registry-owned
entries go through the claude CLI, plain folders through `DeleteArtifact`.
It powers `adev clean` (dry-run by default, `--apply` removes) and the TUI's
clean list.

### How the TUI works

`internal/tui` is a Bubble Tea program with a root model that owns the screen
and delegates to one sub-model per state: the intro animation, then the
dashboard. The dashboard (`dashboard.go`) is a single model holding:

- **Views 1-5** (paths / skills / plugins / marketplaces / doctor), switched
  with the number keys. The paths view drills into a config dir on enter;
  enter again opens a full-screen detail page. The marketplaces view drills
  into a catalog with `i` (and installs the selected entry with `i` again).
  Group rows carry the drift badge (`=` identical, `≠` drifted). The doctor
  view lists findings with fix-hint detail pages, and `c` flips it to the
  clean candidates, where `d` removes the selection after a y/n confirm.
  `previews.go` builds the content shared by the right preview panel and the
  detail pages.
- **Modes**: normal navigation, the footer input line (`o` changes the scan
  root, `a` adds a marketplace from a source), and the confirm prompt (`d`
  asks y/n; deleting a whole config dir demands its name typed back). `t`
  toggles an installed plugin without confirm.
- **Async work**: scans and mutations run off the update loop as `tea.Cmd`s;
  a finished mutation always triggers a rescan of the current root.

Every operation added to the TUI must ship with its mirror command (with
`--json` where it reports data), and vice versa; the shared logic lives in
`discovery`/`manage`, never duplicated. `ADEV_TUI_LOG=<file>` routes the debug
log (including every key received) to a file, since a TUI owns the terminal.

### How the e2e suite works

The TUI is exercised end to end in real pseudo-terminals: `make e2e` runs
every flow in `e2e/*.exp` through `expect` against a throwaway fixture home
(`e2e/fixture.sh`), so the real config is never touched. `make e2e E2E=<name>`
runs one flow; `E2E_VERBOSE=1` prints the raw pty stream; a failing test keeps
its fixture dir and the `ADEV_TUI_LOG` key trace.

The rules live in `e2e/lib/harness.tcl` and are non-negotiable when writing
tests: event-driven awaits only (never sleep-then-send: keys sent before raw
mode arrive late and coalesced), answer the startup terminal queries (OSC 11,
CSI 6n) or the app blocks, size the pty explicitly, and use markers that
lipgloss renders as one styled segment. Extend the suite whenever the TUI
changes.

## Release flow

`make release` only builds locally into `dist/`; it does not publish. Publishing
is driven by `.github/workflows/release.yml`, which triggers on pushing a tag
`vX.Y.Z`. The workflow runs `make release VERSION=<tag>` (single source of truth
for the build matrix), then publishes the binaries and `checksums.txt` to a
GitHub Release. `install.sh` / `install.bat` download the matching asset from the
latest release, and `adev update` self-updates the same way. To cut a release:

```bash
git tag vX.Y.Z && git push origin vX.Y.Z
```

## Conventions

- **No em-dashes or en-dashes** anywhere in docs, skills, or code; use commas,
  colons, periods, or parentheses. Hyphens in identifiers (`adev-cli`) are fine.
- Code, docs, commit messages, and PRs are in English.
- Module path is `agentic-developer`; `-ldflags` set
  `agentic-developer/internal/cli.Version`.
- Never commit secrets. `bin/`, `dist/`, and `tmp/` are gitignored.
- Bundled skills follow [PHILOSOPHY.md](PHILOSOPHY.md): a `SKILL.md` index under
  200 lines, `references/` vs `assets/`, deterministic scripts, a preflight.

## Verifying setup

Run `setup` against a throwaway HOME so it does not write to your real config:

```bash
go build -o /tmp/adev ./cmd/adev
rm -rf /tmp/fh && mkdir -p /tmp/fh/.claude
HOME=/tmp/fh /tmp/adev setup
find /tmp/fh/.claude/skills -type f
```

## Known limitation

`adev new skill` cannot yet scaffold directly into a plugin's
`plugins/<plugin>/skills/` folder; it always writes to `.<harness>/skills/`. See
the README roadmap.
