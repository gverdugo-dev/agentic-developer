# adev command reference

How every `adev` command parses its arguments and what it does on disk.

## Contents

- Synopsis
- Arguments
- `adev new` — scaffold an artifact
- `adev delete` — remove an artifact
- `adev setup` — install adev's own skills
- Harness detection
- Scope and placement
- What gets scaffolded
- Errors and exit behavior
- Examples

## Synopsis

```
adev new    <artifact> <name> [scope] [harness] [--force]
adev delete <artifact> <name> [scope] [harness]
adev setup  [harness]
```

`new` and `delete` are positional: the first three words are always
`<verb> <artifact> <name>`. `scope` and `harness` are positional optionals in
that order — to pass `harness` you must also pass `scope` before it. `--force`
is a flag, so it can appear anywhere in the command.

## Arguments

| Argument   | Required | Values                                    | Default                |
| ---------- | -------- | ----------------------------------------- | ---------------------- |
| `artifact` | yes      | `skill`, `plugin`, `plugin-marketplace`   | —                      |
| `name`     | yes      | any name without `/`, `\`, or `..`        | —                      |
| `scope`    | no       | `project`, `local`                        | `project`              |
| `harness`  | no       | `claude`, `codex`, `opencode`             | auto-detected          |
| `--force`  | no       | flag                                      | off (refuse if exists) |

`name` becomes the artifact's root folder, so it is validated: an empty name, or
one containing `/`, `\`, or `..`, is rejected to keep the write inside the target
directory.

## `adev new` — scaffold an artifact

Creates the artifact's folder structure at the resolved destination (see
*Scope and placement*).

Behavior:

1. Resolve the destination directory from `scope` + `harness`.
2. If the artifact root (`<dest>/<name>`) already exists:
   - without `--force` → error, nothing is written;
   - with `--force` → the existing root is **removed and recreated from
     scratch** (a complete overwrite, not a merge).
3. Create the layout: paths ending in `/` become directories; the rest become
   empty files. Parent directories are created as needed.

The default refusal is deliberate — it prevents clobbering an artifact you have
already started. Reach for `--force` only when you mean to discard the current
contents.

## `adev delete` — remove an artifact

Removes a previously scaffolded artifact: the single root folder `<dest>/<name>`
at the resolved destination. The whole scaffold lives under that one folder, so
removing it is enough.

Safeguards:

- the `name` validation above applies (no empty, `/`, `\`, or `..`);
- if the target does not exist, or is not a directory, it errors instead of
  deleting anything.

## `adev setup` — install adev's own skills

Installs the skills bundled in the binary (`adev-cli`, `adev-skill-builder`,
`adev-plugin-builder`, `adev-plugin-marketplace-builder`) so the agent learns
how to use the tool.

- It **always targets the user's config (home)**, never the project.
- The harness comes from the optional argument, or is auto-detected from the
  user's home when omitted.
- Skills are written to `<home>/.<harness>/skills/`. Existing files are
  overwritten, so re-running `setup` doubles as an update.

## Harness detection

When `harness` is omitted, adev detects it by looking for a tool-specific config
directory:

| Harness  | Marker directory |
| -------- | ---------------- |
| Claude   | `.claude`        |
| Codex    | `.codex`         |
| opencode | `.opencode`      |

Rules:

- Detection looks **only in the base directory** where the command will write —
  the project dir for `new`/`delete` (project scope), the user's home for
  `setup`. It never infers a harness from somewhere else and then writes a
  config dir where there wasn't one.
- `AGENTS.md` is **not** a marker: it is the cross-tool open standard, so its
  presence doesn't identify a single harness.
- When several markers coexist, a fixed priority decides:
  **Claude → Codex → opencode**.
- If no marker is found, the command errors and asks for an explicit harness.
- Passing `harness` explicitly bypasses detection entirely.

## Scope and placement

The destination is built as:

```
<scope base dir> / .<harness> / <artifact subdir> / <name>
```

- **Scope base dir**: `project` → current directory; `local` → the user's home.
- **Artifact subdir**: where each artifact type lives inside the harness dir.

| Artifact             | Subdir         | Destination (Claude, project scope) |
| -------------------- | -------------- | ----------------------------------- |
| `skill`              | `skills`       | `.claude/skills/<name>/`            |
| `plugin`             | `plugins`      | `.claude/plugins/<name>/`           |
| `plugin-marketplace` | `marketplaces` | `.claude/marketplaces/<name>/`      |

Example: `adev new skill foo local` on a Claude setup creates
`~/.claude/skills/foo/`.

## What gets scaffolded

The folder layout per harness and artifact is defined in the binary's embedded
config. Not every harness supports every artifact; asking for an unsupported
pair errors.

| Harness  | skill | plugin | plugin-marketplace |
| -------- | :---: | :----: | :----------------: |
| Claude   |  yes  |  yes   |        yes         |
| Codex    |  yes  |   —    |         —          |
| opencode |  yes  |  yes   |         —          |

Typical Claude layouts:

- **skill** → `SKILL.md` + `references/` + `assets/` + `scripts/`
- **plugin** → `.claude-plugin/plugin.json` + `commands/` + `agents/` +
  `skills/` + `hooks/`
- **plugin-marketplace** → `.claude-plugin/marketplace.json` + `plugins/`

## Errors and exit behavior

`adev` exits non-zero and prints a single error on any failure, including:

- unknown verb, artifact, scope, or harness;
- missing required arguments;
- no harness detected and none given;
- an unsupported harness/artifact pair;
- `new` over an existing artifact without `--force`;
- `delete` of something that doesn't exist or isn't a directory.

## Examples

```bash
# Skill in the current project, harness auto-detected
adev new skill my-new-skill

# Plugin on the whole machine (under the user's home)
adev new plugin my-new-plugin local

# Skill forced into the opencode layout, in the current project
adev new skill my-new-skill project opencode

# Overwrite an existing skill completely
adev new skill my-new-skill --force

# Remove a skill
adev delete skill my-new-skill

# Install adev's skills into the detected harness (home)
adev setup

# ... into a specific harness
adev setup claude
```
