---
name: adev-plugin-builder
description: "Guides building a well-formed plugin the opinionated adev way — the plugin boundary, plugin.json, generic single-task agents, and the skills that orchestrate them. Use when filling in a plugin scaffolded by `adev new plugin`, writing a plugin.json or its agents, deciding what belongs in a plugin, or when the user asks how to build a good plugin."
---

# adev-plugin-builder

Turns an empty plugin scaffold (from `adev new plugin`) into a well-formed plugin
that follows the adev house style. This skill is the orchestrator: it walks you
through the build and defers the detailed rules to its references.

## Preflight

Before anything else, run the environment check and fix what it reports:

```bash
bash scripts/setup.sh      # Unix/macOS
scripts\setup.bat          # Windows
```

## What a plugin is (the house style)

A plugin is a **logical container of AI artifacts** that belong together —
because they're coherent with each other, or serve a specific profile of people
(a process, a team, a department). Its two key pieces are **skills** (the
processes) and **agents** (the generic, single-task workers the skills
coordinate). Agents are what make a plugin's skills more powerful than a
standalone skill. Full principles: [references/philosophy.md](references/philosophy.md).

## Build workflow

Copy this checklist and work top to bottom:

```
- [ ] 1. Define the plugin boundary (process / team / audience)
- [ ] 2. Decompose into skills (processes) and agents (workers)
- [ ] 3. Write plugin.json (the manifest)
- [ ] 4. Build the agents: generic, single-task, judgment-bearing
- [ ] 5. Build the skills, wired to launch those agents
- [ ] 6. Verify against the checklist
```

**1. Define the boundary.** State what ties these artifacts together in one
sentence: the process, team, or audience they serve. That sentence is the test
for what belongs in the plugin and what doesn't.

**2. Decompose.** List the **skills** (each an ordered process) and the **agents**
(generic workers a skill fans work out to). For each skill, note which agents it
orchestrates. Map which steps are deterministic (→ scripts), which need judgment
(→ agents), and which are independent (→ parallel). See
[references/agents.md](references/agents.md) for what makes a good agent.

**3. Write plugin.json.** The manifest at `.claude-plugin/plugin.json`. Copy
[assets/plugin.json](assets/plugin.json) and fill it in. Layout details:
[references/structure.md](references/structure.md).

**4. Build the agents.** One file per agent in `agents/`. Keep each **generic and
single-task**: it executes some actions and returns a decision; the skill passes
the specifics at call time. Copy [assets/agent.md](assets/agent.md) per agent.
Rules: [references/agents.md](references/agents.md).

**5. Build the skills.** Use the **`adev-skill-builder`** skill for each one.
Wire each skill to launch the plugin's agents by their namespaced type
(`<plugin>:<agent>`) and to own the orchestration and the final decision. How
skills and agents connect: [references/structure.md](references/structure.md).

**6. Verify.** Run the checklist at the bottom of
[references/philosophy.md](references/philosophy.md).

## What this skill ships

- [references/philosophy.md](references/philosophy.md) — plugins and subagents in
  full, plus the house style its artifacts inherit.
- [references/structure.md](references/structure.md) — the plugin folder layout,
  `plugin.json`, and how skills launch agents.
- [references/agents.md](references/agents.md) — how to write a generic,
  single-task agent.
- [references/setup.md](references/setup.md) — this skill's own requirements.
- [assets/plugin.json](assets/plugin.json), [assets/agent.md](assets/agent.md) —
  templates to fill in.

To package or distribute the plugin through a marketplace, use
`adev-plugin-marketplace-builder`.
