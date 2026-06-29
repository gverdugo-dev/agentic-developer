# Plugin marketplace reference

The standard Claude Code marketplace format: the catalog file, plugin sources,
versioning, and hosting. This is convention, not house opinion, so follow it.

## Contents

- The file and folder layout
- marketplace.json schema
- Plugin entries
- Plugin sources
- Versioning
- Hosting and distribution
- Reserved names and gotchas

## The file and folder layout

`adev new plugin-marketplace <name>` scaffolds:

```
<marketplace>/
├── .claude-plugin/
│   └── marketplace.json    # the catalog
└── plugins/                # plugins hosted in this same repo (optional)
    └── <plugin>/           # each a plugin (see adev-plugin-builder)
```

The **marketplace root** is the directory containing `.claude-plugin/`. Relative
plugin paths resolve from there (not from inside `.claude-plugin/`).

## marketplace.json schema

Required fields:

| Field     | Type   | Description                                              |
| --------- | ------ | ------------------------------------------------------- |
| `name`    | string | Marketplace id, kebab-case, public-facing, not reserved |
| `owner`   | object | Maintainer: `name` (required), `email` (optional)       |
| `plugins` | array  | List of plugin entries (see below)                      |

Useful optional fields:

- `description`: one line about the catalog.
- `metadata.pluginRoot`: a base dir prepended to relative sources, so
  `"./plugins"` lets you write `"source": "formatter"` instead of
  `"source": "./plugins/formatter"`.
- `$schema`: JSON Schema URL for editor autocomplete (ignored at load time).

```json
{
  "name": "company-tools",
  "owner": { "name": "DevTools Team", "email": "devtools@example.com" },
  "plugins": [
    {
      "name": "code-formatter",
      "source": "./plugins/formatter",
      "description": "Automatic code formatting on save",
      "version": "2.1.0"
    }
  ]
}
```

## Plugin entries

Each entry needs at least a `name` (kebab-case, public-facing) and a `source`.
Common optional fields: `description`, `version`, `author` (`name` required),
`homepage`, `repository`, `license`, `keywords`, `category`, `tags`.

## Plugin sources

The `source` tells Claude Code where to fetch that plugin from:

| Source        | Shape                                                  | Use for                                  |
| ------------- | ------------------------------------------------------ | ---------------------------------------- |
| Relative path | `"./plugins/x"` (string, must start with `./`)         | plugin in this same repo                 |
| `github`      | `{ "source": "github", "repo": "owner/repo" }`         | a plugin in a GitHub repo                |
| `url`         | `{ "source": "url", "url": "https://…​.git" }`         | any git host (GitLab, Bitbucket, …)      |
| `git-subdir`  | `{ "source": "git-subdir", "url": …, "path": "…" }`    | a plugin inside a monorepo subdirectory  |
| `npm`         | `{ "source": "npm", "package": "@org/x" }`             | a plugin published to npm                |

The git sources (`github`, `url`, `git-subdir`) accept optional `ref` (branch or
tag) and `sha` (full 40-char commit). When both are set, `sha` wins.

A single marketplace can mix sources: list local plugins by relative path and
external ones by `github`/`url`, each pinned independently.

## Versioning

- **Pin a `version` string** (in the entry or the plugin's `plugin.json`): users
  only get updates when you bump it. Bump on every release.
- **Omit `version`** (git-hosted): every commit counts as a new version, users
  track the latest commit automatically.

Pick pinned versions for stable, controlled rollouts; omit for fast-moving
internal catalogs.

## Hosting and distribution

**GitHub is the recommended host** (version control, issues, collaboration):

1. Create a repository for the marketplace.
2. Add `.claude-plugin/marketplace.json`.
3. Share: users run `/plugin marketplace add <owner>/<repo>`, then
   `/plugin install <plugin>@<marketplace-name>`.

Other git hosts work the same via the git URL. Publish updates by pushing; users
refresh with `/plugin marketplace update`.

> **Marketplace source vs plugin source** are independent: where the catalog
> itself is fetched from vs where each listed plugin is fetched from. The
> marketplace can live in one repo and list plugins from a dozen others.

## Reserved names and gotchas

- **Reserved names** (blocked for third parties): `claude-code-marketplace`,
  `claude-code-plugins`, `claude-plugins-official`, `anthropic-marketplace`,
  `anthropic-plugins`, and other names impersonating official catalogs. Pick a
  distinct, team/company-specific name.
- **One name per user**: adding a second marketplace with the same `name`
  replaces the first. Put all your plugins under one `marketplace.json`.
- **Relative paths + URL distribution don't mix**: relative `./` sources only
  resolve when users add the marketplace from a git or local source. If you
  distribute a direct URL to the JSON, only that file is downloaded and relative
  paths silently fail, so use `github`/`url`/`npm` sources instead.
- **No `../`**: a plugin can't reference files outside the marketplace root;
  plugins are copied to a cache on install.
