---
name: adev-skill-builder
description: "Guides building a well-formed agent skill the opinionated adev way: folder structure, frontmatter, deterministic scripts, references vs assets, a preflight check, and subagent orchestration. Use when filling in a skill scaffolded by `adev new skill`, writing or restructuring a SKILL.md, or when the user asks how to build a good skill."
---

# adev-skill-builder

Turns an empty skill scaffold (from `adev new skill`) into a well-formed skill
that follows the adev house style. This skill is the orchestrator: it walks you
through the build and defers the detailed rules to its references.

## Preflight

Before anything else, run the environment check and fix what it reports:

```bash
bash scripts/setup.sh      # Unix/macOS
scripts\setup.bat          # Windows
```

## The house style, in one breath

A skill is an **operator**: one job, done the same way every time. Make every
measurable step **deterministic** (a script), reserve the agent for **judgment**,
keep `SKILL.md` a thin **index**, and **fail fast** if the environment isn't
ready. Full principles in [references/philosophy.md](references/philosophy.md).

## Build workflow

Copy this checklist and work top to bottom:

```
- [ ] 1. Define the job + 2-3 concrete use cases
- [ ] 2. Decide: skill or plugin?
- [ ] 3. Scaffold it: adev new skill <name> [--scope s] [--harness h]
- [ ] 4. Map the steps: deterministic vs judgment, parallel vs dependent
- [ ] 5. Write the deterministic scripts (+ scripts/CLAUDE.md index)
- [ ] 6. Add the preflight (scripts/setup.sh|.bat + references/setup.md)
- [ ] 7. Split knowledge: references/ vs assets/
- [ ] 8. Write SKILL.md as an index (<200 lines) + frontmatter
- [ ] 9. Verify against the checklist
```

**1. Define the job + use cases.** State the single job in one sentence, then
write 2-3 concrete tasks the skill must handle. If you can't, the scope is wrong.

**2. Skill or plugin?** If the process needs its own subagents or groups several
related skills, it's a plugin. Otherwise a skill. See
[references/philosophy.md](references/philosophy.md) (§3.1).

**3. Scaffold it with adev.** Let adev create the folder skeleton, don't make it
by hand:

```bash
adev new skill <name> [--scope s] [--harness h]
```

This creates `SKILL.md` plus empty `references/`, `assets/`, and `scripts/` in the
right harness dir. The steps below fill that skeleton in. adev refuses to
overwrite an existing skill unless you add `--force`. (Full command surface:
the `adev-cli` skill. Remove a skill with `adev delete skill <name>`.)

**4. Map the steps.** For each step decide: is it *deterministic* (→ a script the
skill calls) or *judgment* (→ the agent, or a subagent for a clean context)? Mark
which steps are independent so they can run in parallel.

**5. Write the deterministic scripts.** Dependency-free CLIs, JSON in / JSON out,
secrets never committed. Keep a `scripts/CLAUDE.md` index up to date. Rules:
[references/philosophy.md](references/philosophy.md) (§1).

**6. Add the preflight.** Every skill verifies its own requirements first. Copy
[assets/setup.sh](assets/setup.sh) and [assets/setup.md](assets/setup.md) into
the new skill, then customize the checks for what it needs.

**7. Split knowledge.** Pure knowledge the agent reads → `references/`. Artifacts
a step *uses* to produce output (templates, data) → `assets/`.

**8. Write SKILL.md.** Under 200 lines, an index that points to the references.
Frontmatter and body rules: [references/authoring.md](references/authoring.md).

**9. Verify.** Run the checklist at the bottom of
[references/philosophy.md](references/philosophy.md) before calling it done.

## What this skill ships

- [references/philosophy.md](references/philosophy.md): the principles in full.
- [references/authoring.md](references/authoring.md): how to write SKILL.md
  (frontmatter, structure, progressive disclosure).
- [references/setup.md](references/setup.md): this skill's own requirements.
- [assets/setup.sh](assets/setup.sh), [assets/setup.md](assets/setup.md):
  preflight templates to drop into the skill you're building.

When the skill is done, fill its artifacts using the matching builders for any
plugin or marketplace it ships: `adev-plugin-builder`,
`adev-plugin-marketplace-builder`.
