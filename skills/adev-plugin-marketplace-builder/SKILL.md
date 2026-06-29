---
name: adev-plugin-marketplace-builder
description: "Guides building a plugin-marketplace the standard way: the marketplace.json catalog, plugin sources, versioning, and hosting it on GitHub or another git remote. Use when filling in a marketplace scaffolded by `adev new plugin-marketplace`, writing a marketplace.json, listing plugins for a team or company, or when the user asks how to build or host a plugin marketplace."
---

# adev-plugin-marketplace-builder

Turns an empty marketplace scaffold (from `adev new plugin-marketplace`) into a
working plugin catalog. A marketplace is **not very opinionated** (it follows
the standard Claude Code format), so this skill leans on the official conventions
and keeps the house touch light. It is the orchestrator; the detail lives in the
references.

## Preflight

Before anything else, run the environment check and fix what it reports:

```bash
bash scripts/setup.sh      # Unix/macOS
scripts\setup.bat          # Windows
```

## What a plugin marketplace is

A plugin marketplace is a **remote catalog where plugins live**: a
`.claude-plugin/marketplace.json` that lists plugins and where to fetch each one.
It is usually scoped to a **team or a company**, and the good practice is to host
it on **GitHub or another git remote** so it gets version control and updates for
free. Users add it with `/plugin marketplace add <owner>/<repo>` and install
plugins from it.

## Build workflow

Copy this checklist and work top to bottom:

```
- [ ] 1. Decide the catalog's scope (team / company) and which plugins it lists
- [ ] 2. Scaffold it: adev new plugin-marketplace <name> [--scope s] [--harness h]
- [ ] 3. Write marketplace.json (name, owner, plugins)
- [ ] 4. Set each plugin's source and versioning
- [ ] 5. Host it on a git remote (GitHub recommended)
- [ ] 6. Validate
```

**1. Scope the catalog.** Decide who it serves (a team, a company) and which
plugins it lists. One marketplace can reference plugins from many repos.

**2. Scaffold it with adev.** Let adev create the skeleton, don't make it by
hand:

```bash
adev new plugin-marketplace <name> [--scope s] [--harness h]
```

This creates `.claude-plugin/marketplace.json` plus an empty `plugins/` folder.
The steps below fill it in. adev refuses to overwrite an existing marketplace
unless you add `--force`. (Full command surface: the `adev-cli` skill. Remove one
with `adev delete plugin-marketplace <name>`.)

**3. Write marketplace.json.** A `name` (kebab-case, not a reserved name), an
`owner`, and a `plugins` array where each entry has at least a `name` and a
`source`. Copy [assets/marketplace.json](assets/marketplace.json). Full schema:
[references/marketplace.md](references/marketplace.md).

**4. Set sources and versioning.** For each plugin, choose where it's fetched
from (a relative path (`./plugins/<name>` in this same repo), a `github` repo, a
git `url`, a `git-subdir`, or `npm`) and decide versioning: pin a `version`
string, or omit it to track the git commit SHA. See
[references/marketplace.md](references/marketplace.md).

**5. Host it.** Push the repo to GitHub (recommended) or another git host. Share
it: users run `/plugin marketplace add <owner>/<repo>`; you publish updates by
pushing, and users refresh with `/plugin marketplace update`.

**6. Validate.** `marketplace.json` is valid JSON; `name` isn't reserved;
relative `source` paths start with `./`. Note: relative paths only resolve when
users add the marketplace from a git/local source, not from a direct URL to the
JSON.

## What this skill ships

- [references/marketplace.md](references/marketplace.md): the marketplace.json
  schema, plugin sources, versioning, hosting, and how users add/update it.
- [references/setup.md](references/setup.md): this skill's own requirements.
- [assets/marketplace.json](assets/marketplace.json): a template to fill in.

To build the plugins this marketplace lists, use `adev-plugin-builder` (and
`adev-skill-builder` for their skills).
