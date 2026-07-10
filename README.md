# agentic-dev (`adev`)

A small, agentic-first CLI, invoked as **`adev`**, for scaffolding, discovering
and managing AI coding-agent artifacts (**skills**, **plugins**, and
**plugin-marketplaces**) with the correct folder layout for the harness you use
(Claude Code, Codex, opencode).

It ships with opinionated, built-in layouts so that creating a well-formed
artifact is a single command, even for non-technical users. It also bundles its
own skills, so after `adev setup` your agent knows how to drive the tool and how
to build well-formed artifacts. And run bare, `adev` opens a lazygit-style TUI
that shows every artifact on your machine and lets you act on it.

## Features

- **Interactive TUI**: `adev` with no arguments opens a terminal dashboard that
  scans for harness config dirs and browses everything inside them.
- **Discovery**: `adev scan` and `adev list` find every config dir under a
  folder (gitignore-aware) and group the artifacts across them, with `--json`
  output for scripts.
- **Management**: `adev rm`, `adev plugin` and `adev marketplace` delete
  artifacts, install, toggle or uninstall plugins, and add or remove
  marketplaces; the same operations the TUI offers.
- **Duplicate detection**: artifacts are content-hashed, so copies of the same
  artifact across config dirs show up as identical (`=`) or drifted (`≠`), and
  `adev list --duplicates` filters to them.
- **Health checks**: `adev doctor` reports broken artifacts (missing or invalid
  manifests, ghost plugins, dead marketplace sources) with fix hints.
- **Cleanup**: `adev clean` lists the removable dead weight (stale cached
  plugin versions, orphaned caches, dead marketplaces, unloadable artifacts)
  with its reclaimable size; dry-run by default, `--apply` removes it.
- **Create and delete** skills, plugins and plugin-marketplaces.
- **Bundled skills**: `adev setup` installs adev's own skills into your harness,
  teaching the agent how to use the tool and an opinionated way to build
  artifacts.
- **Harness auto-detection** from the project's config folder (`.claude`,
  `.codex`, `.opencode`), with an optional explicit override.
- **Scoped placement**: scaffold into the current project or into the whole
  machine (the user's home).
- **Correct destinations**: artifacts are placed inside the harness config dir
  (e.g. `.claude/skills/<name>/`), not loose in the working directory.
- **Self-contained binary**: the layout config and the bundled skills are
  embedded at build time, so there are no external files to ship.

## Install

Prebuilt binaries, no Go toolchain required.

**macOS / Linux**

```bash
curl -fsSL https://raw.githubusercontent.com/gverdugo-dev/agentic-developer/main/install.sh | sh
```

**Windows** (PowerShell or `cmd`)

```bat
curl -fsSL -o install.bat https://raw.githubusercontent.com/gverdugo-dev/agentic-developer/main/install.bat && install.bat
```

The installer detects your OS/arch, downloads the matching binary from the
[latest release](https://github.com/gverdugo-dev/agentic-developer/releases),
and drops `adev` on your PATH. Override the location with `ADEV_INSTALL_DIR`.

Then install adev's bundled skills into your harness:

```bash
adev setup          # harness auto-detected from your home (~/.claude, ...)
adev setup claude   # or target a specific harness
```

`setup` always writes to the **user's** config (home), never the project.

To upgrade later, run `adev update`: it self-updates to the latest release in
place. Check your version with `adev version`.

> Building from source instead? See [Development](#development).

## Usage

```
adev                                                  # open the TUI (terminal only)
adev new <artifact> <name> [--scope s] [--harness h] [--force]
adev delete <artifact> <name> [--scope s] [--harness h]
adev scan [path] [--json]
adev list <skills|plugins|marketplaces> [path] [--json] [--duplicates]
adev doctor [path] [--json]
adev clean [path] [--json] [--apply]
adev rm <absolute-path> [--yes]
adev plugin <install|enable|disable|uninstall> <name@marketplace>
adev marketplace <add <source>|remove <name>>
adev setup [harness]
adev update
adev version
```

Every invocation is a command. `new` and `delete` take two positional arguments
(the artifact and its name) plus optional flags.

| Argument    | Required | Values                                      | Default                  |
| ----------- | -------- | ------------------------------------------- | ------------------------ |
| `artifact`  | yes      | `skill`, `plugin`, `plugin-marketplace`     | (none)                   |
| `name`      | yes      | any name without `/`, `\` or `..`           | (none)                   |
| `--scope`   | no       | `project`, `local`                          | `project`                |
| `--harness` | no       | `claude`, `codex`, `opencode`               | auto-detected            |
| `--force`   | no       | overwrite an existing artifact (`new` only) | off (refuse if exists)   |

> Flags may appear before, after, or between the positional arguments, in any
> order: `adev new skill foo --harness opencode` and
> `adev new --harness opencode skill foo` are equivalent.

By default, `new` refuses to scaffold over an artifact whose folder already
exists, to avoid clobbering work. Pass `--force` to remove the existing folder
and recreate it from scratch.

### `adev setup`

`setup` installs adev's own bundled skills into the user's harness, so the agent
learns how to use the tool. It always targets the user's config (home), never
the project. The harness is taken from the optional argument, or auto-detected
from your home when omitted.

```
adev setup [harness]
```

### Examples

```bash
# Install adev's skills into the detected harness (home)
adev setup

# Skill in the current project, harness auto-detected
adev new skill my-new-skill

# Plugin on the whole machine (under the user's home)
adev new plugin my-new-plugin --scope local

# Skill forced into the opencode layout, in the current project
adev new skill my-new-skill --harness opencode

# Remove a skill
adev delete skill my-new-skill

# Overwrite an existing skill completely
adev new skill my-new-skill --force
```

## The TUI

Running `adev` with no arguments on a terminal opens the dashboard (the lazygit
model: the bare binary is the interactive tool). It scans the directory you
started it in, always adds your home config dirs (`~/.claude`, `~/.codex`,
`~/.opencode`), and browses the results in five views:

| View               | Shows                                                        |
| ------------------ | ------------------------------------------------------------ |
| `1` paths          | every discovered config dir; enter drills into its artifacts |
| `2` skills         | every skill across all config dirs, grouped by name          |
| `3` plugins        | every plugin identity (`name@marketplace`), grouped          |
| `4` marketplaces   | every registered marketplace, grouped by name                |
| `5` doctor         | every doctor finding, with a fix hint on its detail page; `c` flips to the removable clean candidates |

The left panel is the browse list, the right panel previews the selection, and
enter opens a full-screen detail page. Keys:

| Key             | Action                                                         |
| --------------- | -------------------------------------------------------------- |
| `1`-`5`         | switch view                                                    |
| `j`/`k`, arrows | move the selection (or scroll the focused panel / open page)   |
| `tab`, `h`/`l`  | switch focus between the list and the preview panel            |
| `enter`         | drill into a config dir, or open the detail page               |
| `esc`           | close the page / leave the drill                               |
| `o`             | change the scan root (footer input; absolute path, `~` works)  |
| `d`             | delete the selection (y/n; config dirs demand the typed name)  |
| `t`             | enable/disable the selected installed plugin                   |
| `c`             | doctor: list the clean candidates; `d` removes one (y/n)       |
| `i`             | marketplaces: open the catalog; on a catalog entry, install it |
| `a`             | marketplaces: add a marketplace from a source (footer input)   |
| `q`, `ctrl+c`   | quit                                                           |

Group rows that live in more than one place carry a content badge, `=` when
every copy is identical and `≠` when they drifted, and single or unhashed
groups show `×N`; the detail page lists each location's hash and how many
files differ. Aggregated rows cannot be deleted or toggled from those views;
the paths view is where you pick which copy.

## Discovering and managing artifacts

Everything the TUI does is also a plain command, so scripts and CI keep the
same capability. `scan` and `list` take `--json` for machine-readable output.

```bash
# Every harness config dir under a folder (default: the current dir)
adev scan ~/dev --json

# Every skill under a folder, grouped by name with its locations
adev list skills ~/dev

# Only the skills living in more than one place (with = / ≠ drift state)
adev list skills ~/dev --duplicates

# Report every broken artifact under a folder, with fix hints
adev doctor ~/dev

# List the removable dead weight (stale cache versions, orphans, dead
# marketplaces, broken artifacts) and what it reclaims; --apply removes it
adev clean ~/dev
adev clean ~/dev --apply

# Delete an artifact folder, or a whole config dir (typed-name confirm)
adev rm /abs/path/.claude/skills/old-skill

# Install, toggle or uninstall a plugin (delegated to the claude CLI)
adev plugin install personal@gonzaloverdugo
adev plugin disable personal@gonzaloverdugo
adev plugin uninstall personal@gonzaloverdugo

# Add or remove a registered marketplace (delegated to the claude CLI)
adev marketplace add gverdugo-dev/some-marketplace
adev marketplace remove gonzaloverdugo
```

Discovery walks the tree the way git would: an embedded default ignore list
(dependency folders, caches, build output) applies everywhere, every
`.gitignore` applies to its own subtree, symlinks are not followed, and config
dirs are leaves (nothing inside one is scanned twice). Claude config dirs are
read through Claude Code's own plugin registry when one exists (the same data
its `/plugins` screen shows, including enabled state and versions); otherwise
the folder layout is listed.

Deletes are guarded: `adev rm` (and the TUI's `d`) refuse to touch anything
that is not a harness config dir or inside one. Operations on installed
plugins and registered marketplaces are delegated to the `claude` CLI, which
owns that registry and its cache; adev never edits those JSON files by hand.

## How it works

The command flows through three layers:

1. **`main`** (`cmd/adev`) wires the program and maps any error to a
   non-zero exit code.
2. **`cli`** (`internal/cli`) is the boundary: every invocation is a `Command`
   that parses its own flags and runs itself; `Run` dispatches to the one named
   on the command line (or opens the TUI when there is none and a terminal is
   attached).
3. The domain packages do the real work, shared by the CLI and the TUI:
   - **`scaffolding`** (`internal/scaffolding`): the typed model, harness
     detection, the embedded layout config, and the create/remove engine.
   - **`harness`** (`internal/harness`): one adapter per harness (marker dir,
     artifact containers, registry reader, manifest validators, operation
     executor), so supporting a new harness is implementing one interface.
   - **`discovery`** (`internal/discovery`): the gitignore-aware scan for
     config dirs, driven by the adapters, plus SKILL.md frontmatter parsing,
     the cross-path grouping, and the content hashes behind drift detection.
   - **`doctor`** (`internal/doctor`): the shared skill checks plus each
     adapter's validators, behind `adev doctor` and the TUI's doctor view.
   - **`clean`** (`internal/clean`): the removal candidates built on discovery
     and doctor, behind `adev clean` and the TUI's clean list.
   - **`manage`** (`internal/manage`): the mutations on discovered resources;
     guarded filesystem deletes, and plugin/marketplace operations forwarded
     to the Claude adapter's `claude` CLI executor.
   - **`tui`** (`internal/tui`): the Bubble Tea dashboard on top of discovery
     and manage.

### Command structure

The `cli` package uses the subcommand pattern (the same shape as the `go` tool):
a small interface plus a registry that maps the first argument to a command.

```go
type Command interface {
	Name() string            // the word typed on the command line
	Synopsis() string        // one-line description for the help listing
	Run(args []string) error // parses the args after the name, then runs
}
```

`new`, `delete`, `setup`, `update`, and `version` are all sibling commands, each
owning the parsing of its own flags. `new` and `delete` share one type
(`scaffoldCmd`) parameterized by a verb, since they have the same grammar and
differ only in create vs remove. Adding a command means implementing `Command`
and registering it; the dispatcher does not change.

### Harness detection

Detection looks for a tool-specific config directory, **not** for `AGENTS.md`:
that file is the cross-tool open standard, so it doesn't identify a single
harness.

| Harness  | Marker directory |
| -------- | ---------------- |
| Claude   | `.claude`        |
| Codex    | `.codex`         |
| opencode | `.opencode`      |

Detection looks **only** in the scope base dir, the same place the artifact
will be written, so it never infers a harness from elsewhere and then
scaffolds a config dir into a project that didn't have one. If no marker is
found there, the command errors and asks you to pass the harness explicitly.
When several markers coexist, a fixed priority (Claude, Codex, opencode) keeps
the result deterministic. An explicit `--harness` flag bypasses detection.

### Scope and placement

The final destination is `scopeBaseDir / placementPrefix / <name>`:

- **Scope** decides the base dir: `project` → current directory; `local` → the
  user's home.
- **Placement prefix** puts every artifact inside the harness config dir, in a
  per-artifact subfolder.

| Artifact             | Placement (Claude example)       |
| -------------------- | -------------------------------- |
| `skill`              | `.claude/skills/<name>/`         |
| `plugin`             | `.claude/plugins/<name>/`        |
| `plugin-marketplace` | `.claude/marketplaces/<name>/`   |

So `new skill foo --scope local` on a Claude setup creates `~/.claude/skills/foo/`.

`adev setup` follows the same principle but is always local: it detects the
harness from your home and installs the bundled skills into
`~/.<harness>/skills/`.

## Bundled skills

`adev setup` installs these skills (embedded in the binary) into the user's
harness:

| Skill                             | What it teaches                                   |
| --------------------------------- | ------------------------------------------------- |
| `adev-cli`                        | How to drive the `adev` command surface           |
| `adev-skill-builder`              | Opinionated way to build a skill                  |
| `adev-plugin-builder`             | Opinionated way to build a plugin                 |
| `adev-plugin-marketplace-builder` | Opinionated way to build a plugin-marketplace     |

The skill sources live under [`skills/`](skills/) and are embedded via
`//go:embed`. Editing a skill and rebuilding updates what `adev setup` installs.

## Configuration

The folder layouts are defined in
[`internal/scaffolding/structures.json`](internal/scaffolding/structures.json)
and embedded into the binary at build time via `//go:embed`.

```json
{
  "harness": {
    "claude": {
      "structures": {
        "skill": [
          "{skill_name}/SKILL.md",
          "{skill_name}/references/",
          "{skill_name}/assets/",
          "{skill_name}/scripts/"
        ]
      }
    }
  }
}
```

- Each path is relative to the artifact root.
- `{placeholder}` tokens are replaced with the artifact name.
- Paths ending in `/` become directories; the rest become empty files.

To change or extend a layout, edit `structures.json` and rebuild.

## Project structure

```
.
├── cmd/adev/main.go                 # Entry point: wiring + exit code
├── internal/
│   ├── cli/                         # One file per command + dispatch
│   │   ├── cli.go                   # Command interface, registry, Run
│   │   ├── scaffold_cmd.go          # `new` + `delete`
│   │   ├── scan.go / list.go        # Discovery commands (--json, --duplicates)
│   │   ├── doctor.go                # `adev doctor`: the health report
│   │   ├── clean.go                 # `adev clean`: dry-run + --apply removals
│   │   ├── rm.go                    # Guarded artifact/config-dir delete
│   │   ├── plugin.go                # install / enable / disable / uninstall
│   │   ├── marketplace.go           # marketplace add / remove
│   │   └── setup.go                 # `adev setup`: install bundled skills
│   ├── scaffolding/                 # Typed model + layout engine
│   │   └── structures.json          # Embedded folder layouts
│   ├── harness/                     # One adapter per harness (claude/codex/opencode)
│   ├── discovery/                   # Gitignore-aware scan + grouping + hashes
│   ├── doctor/                      # Health checks behind `adev doctor`
│   ├── clean/                       # Removal candidates behind `adev clean`
│   ├── manage/                      # Deletes + forwarding to the Claude adapter
│   ├── tui/                         # The Bubble Tea dashboard
│   └── brand/                       # The shared color palette
├── skills/                          # Bundled skills (embedded via go:embed)
│   ├── skills.go                    # Embed + install into a harness
│   ├── adev-cli/
│   ├── adev-skill-builder/
│   ├── adev-plugin-builder/
│   └── adev-plugin-marketplace-builder/
├── e2e/                             # pty e2e suite for the TUI (expect)
├── Makefile                         # build / run / install / e2e targets
└── README.md
```

## Supported harnesses

| Harness  | Status      | Notes                                              |
| -------- | ----------- | -------------------------------------------------- |
| Claude   | Full        | skill, plugin, plugin-marketplace                  |
| Codex    | Partial     | skill only                                          |
| opencode | Partial     | skill, plugin                                       |

## Development

```bash
make build      # build ./bin/adev
make run        # build, then run
make install    # go install ./cmd/adev
go test ./...   # unit tests
make e2e        # pty e2e suite for the TUI (requires `expect`)
go vet ./...    # static checks
gofmt -l .      # formatting check (empty output = clean)
go doc ./internal/scaffolding   # browse the package docs
```

### The e2e suite

The TUI is verified end to end in real pseudo-terminals, driven by `expect`
(preinstalled on macOS, `apt install expect` on Debian/Ubuntu). `make e2e`
builds the binary and runs every flow in `e2e/*.exp` against a throwaway
fixture home, so your real config is never touched. Run a single flow with
`make e2e E2E=view_nav`; set `E2E_VERBOSE=1` to watch the raw pty stream. A
failing test keeps its fixture dir and the `ADEV_TUI_LOG` key trace for
debugging. The harness rules (event-driven awaits, terminal query responder,
pty sizing) live in `e2e/lib/harness.tcl`.

## Known limitations

- **No scaffolding inside a plugin.** `adev new skill` always places the skill in
  `.<harness>/skills/`, not inside a plugin's `plugins/<plugin>/skills/`. Building
  a plugin's skills with `adev` therefore needs a manual move for now. Planned: a
  way to scaffold an artifact *into* a target plugin.

## Roadmap

[~] Create adev skills framework (structure scaffolded; content WIP).
[x] Install skills inside the user's harness on setup (`adev setup`).
[x] Detect folder exists (refuse overwrite unless `--force`)
[ ] Scaffold artifacts inside a target plugin (`plugins/<plugin>/skills/...`).
[x] Install script (install.sh / install.bat) + prebuilt-binary releases.
