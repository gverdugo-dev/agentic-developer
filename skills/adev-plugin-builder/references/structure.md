# Plugin structure

The folder layout of a plugin, the manifest, and how skills connect to agents.

## Contents

- Folder layout
- plugin.json
- agents/
- skills/
- commands/ and hooks/
- How a skill launches an agent
- Other harnesses

## Folder layout

`adev new plugin <name>` scaffolds the Claude plugin layout:

```
<plugin>/
├── .claude-plugin/
│   └── plugin.json     # manifest (name, description, ...)
├── commands/           # slash commands the plugin adds
├── agents/             # generic single-task subagents (one .md per agent)
├── skills/             # the plugin's skills (each a folder with SKILL.md)
└── hooks/              # event hooks
```

A plugin doesn't need every folder. The two that carry the philosophy are
`skills/` and `agents/`; leave `commands/` and `hooks/` empty (or remove them)
until the plugin needs them.

## plugin.json

The manifest lives at `.claude-plugin/plugin.json` and at minimum names the
plugin:

```json
{
  "name": "my-plugin",
  "description": "What the plugin is for and who uses it."
}
```

- `name`: lowercase, hyphens; matches the plugin folder.
- `description`: one line, the boundary (the process / team / audience).
- Optional fields (`version`, `author`, `homepage`) can be added later. With no
  `version`, the plugin is versioned by its git commit.

## agents/

One markdown file per agent: `agents/<agent-name>.md`. Each is a generic,
single-task worker. See [agents.md](agents.md) for the frontmatter and how to
write the prompt.

## skills/

The plugin's skills, each a folder `skills/<skill-name>/` with its own `SKILL.md`
(plus `references/`, `assets/`, `scripts/`). Build each one with the
**`adev-skill-builder`** skill, which owns the per-skill rules.

## commands/ and hooks/

- **`commands/`**: slash commands the plugin exposes. Add only if the plugin
  needs an explicit `/command` entry point.
- **`hooks/`**: event-driven automations (run on tool events, etc.). Add only
  when the plugin must react to harness events.

## How a skill launches an agent

A plugin's skill orchestrates the plugin's agents and owns the final decision.
The skill launches an agent by its **namespaced type**, `<plugin>:<agent>` (for
example `my-plugin:keyword-expert`), via the native subagent integration. The
skill passes the phase's data in the call, the agent runs to a proposal and
returns it, and the skill decides what to do with it.

Because a subagent can't pause for the user, the skill is where you stop between
phases to show a result and wait for approval. Agents are registered when the
plugin is installed; after installing, restart the session so they appear.

## Other harnesses

The layout above is the Claude plugin model (skills + agents). Other harnesses
differ. For example, an opencode plugin is an `index.ts` + `package.json` package, not a
skills/agents tree. `adev new plugin <name> <scope> opencode` scaffolds that
variant. This skill's agent-centric guidance targets the Claude model.
