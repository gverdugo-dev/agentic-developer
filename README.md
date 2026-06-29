# agentic-dev (`adev`)

A small, agentic-first CLI — invoked as **`adev`** — for scaffolding and
removing AI coding-agent artifacts — **skills**, **plugins**, and
**plugin-marketplaces** — with the correct folder layout for the harness you use
(Claude Code, Codex, opencode).

It ships with opinionated, built-in layouts so that creating a well-formed
artifact is a single command, even for non-technical users. It also bundles its
own skills, so after `adev setup` your agent knows how to drive the tool and how
to build well-formed artifacts.

## Features

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

## Installation

Requires Go 1.26+.

```bash
# Build the binary into ./bin/adev
make build

# Install adev into your Go bin (PATH)
make install        # = go install ./cmd/adev

# Or run directly without installing
go run ./cmd/adev <args>
```

Once `adev` is on your PATH, install its bundled skills into your harness:

```bash
adev setup          # harness auto-detected from your home (~/.claude, ...)
adev setup claude   # or target a specific harness
```

`setup` always writes to the **user's** config (home), never the project.

## Usage

```
adev <verb> <artifact> <name> [scope] [harness] [--force]
adev setup [harness]
```

| Argument   | Required | Values                                      | Default                  |
| ---------- | -------- | ------------------------------------------- | ------------------------ |
| `verb`     | yes      | `new`, `delete`                             | —                        |
| `artifact` | yes      | `skill`, `plugin`, `plugin-marketplace`     | —                        |
| `name`     | yes      | any name without `/`, `\` or `..`           | —                        |
| `scope`    | no       | `project`, `local`                          | `project`                |
| `harness`  | no       | `claude`, `codex`, `opencode`               | auto-detected            |
| `--force`  | no       | flag — overwrite an existing artifact       | off (refuse if exists)   |

> Positional optionals: to pass `harness` you must also pass `scope` before it.
> `--force` is a flag, so it can go anywhere in the command.

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
adev new plugin my-new-plugin local

# Skill forced into the opencode layout, in the current project
adev new skill my-new-skill project opencode

# Remove a skill
adev delete skill my-new-skill

# Overwrite an existing skill completely
adev new skill my-new-skill --force
```

## How it works

The command flows through three layers:

1. **`main`** (`cmd/adev`) — wires the program and maps any error to a
   non-zero exit code.
2. **`cli`** (`internal/cli`) — parses and validates the raw args into a typed
   command, then dispatches it.
3. **`scaffolding`** (`internal/scaffolding`) — the domain core: the typed
   model, harness detection, the embedded layout config, and the create/remove
   engine.

### Harness detection

Detection looks for a tool-specific config directory, **not** for `AGENTS.md` —
that file is the cross-tool open standard, so it doesn't identify a single
harness.

| Harness  | Marker directory |
| -------- | ---------------- |
| Claude   | `.claude`        |
| Codex    | `.codex`         |
| opencode | `.opencode`      |

Detection looks **only** in the scope base dir — the same place the artifact
will be written — so it never infers a harness from elsewhere and then
scaffolds a config dir into a project that didn't have one. If no marker is
found there, the command errors and asks you to pass the harness explicitly.
When several markers coexist, a fixed priority (Claude → Codex → opencode) keeps
the result deterministic. An explicit `harness` argument bypasses detection.

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

So `new skill foo local` on a Claude setup creates `~/.claude/skills/foo/`.

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
│   ├── cli/
│   │   ├── cli.go                   # Arg parsing, validation, dispatch
│   │   └── setup.go                 # `adev setup`: install bundled skills
│   └── scaffolding/
│       ├── types.go                 # Typed model + Parse* validators
│       ├── scaffold.go              # Config load + Apply/Remove engine
│       ├── utils.go                 # Detection, placement, lookups
│       └── structures.json          # Embedded folder layouts
├── skills/                          # Bundled skills (embedded via go:embed)
│   ├── skills.go                    # Embed + install into a harness
│   ├── adev-cli/
│   ├── adev-skill-builder/
│   ├── adev-plugin-builder/
│   └── adev-plugin-marketplace-builder/
├── Makefile                         # build / run / install targets
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
go vet ./...    # static checks
gofmt -l .      # formatting check (empty output = clean)
go doc ./internal/scaffolding   # browse the package docs
```

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
[ ] Install script (install.sh / install.bat) for binary distribution.
