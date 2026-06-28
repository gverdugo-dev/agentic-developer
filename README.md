# agentic-dev

A small, agentic-first CLI for scaffolding and removing AI coding-agent
artifacts — **skills**, **plugins**, and **plugin-marketplaces** — with the
correct folder layout for the harness you use (Claude Code, Codex, opencode).

It ships with opinionated, built-in layouts so that creating a well-formed
artifact is a single command, even for non-technical users.

## Features

- **Create and delete** skills, plugins and plugin-marketplaces.
- **Harness auto-detection** from the project's config folder (`.claude`,
  `.codex`, `.opencode`), with a fallback to the user's home, and an optional
  explicit override.
- **Scoped placement**: scaffold into the current project or into the whole
  machine (the user's home).
- **Correct destinations**: artifacts are placed inside the harness config dir
  (e.g. `.claude/skills/<name>/`), not loose in the working directory.
- **Self-contained binary**: the layout config is embedded at build time, so
  there are no external files to ship.

## Installation

Requires Go 1.26+.

```bash
# Build the binary into ./bin/agentic-dev
make build

# Or run directly
go run ./cmd/agentic-dev <args>
```

## Usage

```
agentic-dev <verb> <artifact> <name> [scope] [harness]
```

| Argument   | Required | Values                                      | Default                  |
| ---------- | -------- | ------------------------------------------- | ------------------------ |
| `verb`     | yes      | `new`, `delete`                             | —                        |
| `artifact` | yes      | `skill`, `plugin`, `plugin-marketplace`     | —                        |
| `name`     | yes      | any name without `/`, `\` or `..`           | —                        |
| `scope`    | no       | `project`, `local`                          | `project`                |
| `harness`  | no       | `claude`, `codex`, `opencode`               | auto-detected            |

> Positional optionals: to pass `harness` you must also pass `scope` before it.

### Examples

```bash
# Skill in the current project, harness auto-detected
agentic-dev new skill my-new-skill

# Plugin on the whole machine (under the user's home)
agentic-dev new plugin my-new-plugin local

# Skill forced into the opencode layout, in the current project
agentic-dev new skill my-new-skill project opencode

# Remove a skill
agentic-dev delete skill my-new-skill
```

## How it works

The command flows through three layers:

1. **`main`** (`cmd/agentic-dev`) — wires the program and maps any error to a
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
├── cmd/agentic-dev/main.go          # Entry point: wiring + exit code
├── internal/
│   ├── cli/cli.go                   # Arg parsing, validation, dispatch
│   └── scaffolding/
│       ├── types.go                 # Typed model + Parse* validators
│       ├── scaffold.go              # Config load + Apply/Remove engine
│       ├── utils.go                 # Detection, placement, lookups
│       └── structures.json          # Embedded folder layouts
├── Makefile                         # build / run targets
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
make build      # build ./bin/agentic-dev
make run        # build, then run
go vet ./...    # static checks
gofmt -l .      # formatting check (empty output = clean)
go doc ./internal/scaffolding   # browse the package docs
```

## Roadmap

- [ ] Add the `codex`/`opencode` global config paths (e.g. `~/.config/opencode`)
      to home-level detection.
- [ ] Move `scope` and `harness` to named flags (`--scope`, `--harness`).
- [ ] Flesh out the `codex` and `opencode` artifact layouts.
- [ ] Ship a skill documenting the full opinionated development cycle.
