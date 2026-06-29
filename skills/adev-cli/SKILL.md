---
name: adev-cli
description: "Drives the adev CLI to scaffold and remove AI coding-agent artifacts (skills, plugins, plugin-marketplaces) and to install adev's own skills into a harness. Use when the user wants to create, scaffold, or delete a skill / plugin / plugin-marketplace, run an `adev` command, set adev up in a harness, or asks how adev works."
---

# adev

`adev` is a CLI and an opinionated system for creating AI coding-agent
artifacts. It scaffolds the correct folder layout for a **skill**, **plugin**,
or **plugin-marketplace** inside the right harness config directory (Claude
Code, Codex, opencode), so a well-formed artifact is one command away, with no
guessing where files go or what the manifest is called.

- Repository: https://github.com/gverdugo-dev/agentic-developer
- Documentation: https://github.com/gverdugo-dev/agentic-developer#readme
- Website: _not published yet (TODO)_

## When to use adev

Reach for `adev` instead of creating files by hand whenever you need to:

- start a new skill, plugin, or plugin-marketplace,
- remove one,
- install adev's own skills into a harness (`adev setup`).

## Command quick reference

```
adev new    <artifact> <name> [--scope s] [--harness h] [--force]
adev delete <artifact> <name> [--scope s] [--harness h]
adev setup  [harness]
```

- `artifact`: `skill` | `plugin` | `plugin-marketplace`
- `--scope`: `project` (default) | `local`
- `--harness`: `claude` | `codex` | `opencode` (auto-detected when omitted)
- `--force`: overwrite an existing artifact (`new` only)

Flags may appear before, after, or between the positional arguments.

For the full semantics (argument order, scope and placement, harness detection,
overwrite behavior, the exact destination paths, and what each command
scaffolds), read [references/commands.md](references/commands.md).

## After scaffolding

`adev` only creates the folder structure; the files start empty. To fill them in
well, use the matching builder skill: `adev-skill-builder`,
`adev-plugin-builder`, or `adev-plugin-marketplace-builder`.
