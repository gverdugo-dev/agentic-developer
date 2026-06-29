# Plugins — the opinionated philosophy

The house style for plugins. A plugin contains skills, so the full skill
philosophy still applies to everything inside it — this file focuses on what is
plugin-specific. To build each individual skill, use the `adev-skill-builder`
skill.

## Contents

- Plugins as logical containers
- Subagents (§3.1)
- What the artifacts inside inherit
- Checklist

## Plugins as logical containers

A plugin is a **logical container of AI artifacts**. You reach for one when a set
of artifacts belongs together — either because they are **coherent with each
other**, or because they serve a **specific profile of people**. That is why a
plugin is scoped to a concrete process, a team, or a department: the plugin is the
boundary that says *these artifacts go together*, and the unit in which they ship
and are installed.

Its two key pieces:

- **Skills** — the processes, and the orchestrators that run them.
- **Agents** — the generic, single-task workers the skills coordinate.

Agents are what make a plugin worth it: they let the plugin's skills become **more
powerful** than a standalone skill could be. A standalone skill has no subagents;
a plugin gives its skills that capability, so a skill can fan work out to generic
agents and keep its own context clean while they do the heavy lifting.

**Make a plugin instead of a skill when** the work needs its own subagents, or
when several coherent artifacts serve the same process / team / audience and
should ship and install as one unit.

## §3.1 — Subagents: one task, a clean context

- **One task, then return.** A subagent does a single job and hands its result
  back to the parent; it doesn't run the whole process.
- **A subagent exists for judgment, not for running a CLI.** Its reason to exist
  is an output that's hard to get with code: it executes some actions and *makes a
  decision* from its prompt, then passes it back. Don't spin one up just to run a
  script — if a step is pure deterministic execution, the skill calls the script
  directly.
- **It also keeps the parent's context clean** — a real but secondary benefit.
- **Keep agents generic.** Push the specifics into the prompt the skill hands it,
  not into the agent definition. The more reusable an agent, the better.
- **The skill orchestrates and decides;** the agents are generic workers.

A subagent can't pause to ask the user: it runs to a proposal and returns; the
skill is where you stop, show the user, and wait for approval before the next
phase.

## What the artifacts inside inherit

Everything in the plugin follows the house style:

- **Determinism** — measurable steps are dependency-free scripts (JSON in/out,
  secrets never committed, a `scripts/CLAUDE.md` index).
- **References vs assets** — pure knowledge in `references/`, artifacts a step
  *uses* in `assets/`; each `SKILL.md` under 200 lines.
- **Preflight** — every skill verifies its requirements first via
  `scripts/setup.sh` / `.bat`, documented in `references/setup.md`.

Build the skills with `adev-skill-builder`, which carries these in full.

## Checklist

- [ ] The plugin boundary is one clear sentence (process / team / audience).
- [ ] Everything inside is coherent with that boundary; nothing unrelated.
- [ ] `plugin.json` is present and names the plugin.
- [ ] Agents are generic and single-task; each earns its place with judgment.
- [ ] Skills own the orchestration and the final decision; they launch the
      agents by their namespaced type (`<plugin>:<agent>`).
- [ ] Each skill follows the full house style (built via adev-skill-builder).
