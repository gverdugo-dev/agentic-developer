# The opinionated skill-creation philosophy

The house style behind adev skills. These are the load-bearing ideas; apply
them in order when building a skill.

## Contents

- 0. Skills as operators
- 1. Determinism whenever possible
- 2. References vs assets
- 3. Parallelization and subagents
- 4. Preflight: verify the environment first
- Checklist

## 0. Skills as operators

A skill is the easiest way to automate something today. Treat it as an
*operator*: a small, self-contained worker that owns one job and does it the same
way every time. A single skill can encode a complete, ordered process (the
steps, the decisions, the quality checks) and run it end to end.

## 1. Determinism whenever possible

Prefer code over judgment. Whenever a step can be made deterministic, write a
small script for it instead of asking the agent to reason through it. An agent
guessing its way through an external service is fragile; a tiny CLI that calls it
is repeatable. Before writing the integration, validate the prerequisites (API
key, token, credentials); only build it once it can actually run.

Scripts live in the skill's `scripts/` folder and follow these rules, always:

- **No external dependencies.** Standard library only; no `pip install`.
- **Documented with comments**, and **named constants**, no magic numbers.
- **Config via `.env`** when needed, loaded from one shared helper.
- **Never commit secrets.** Ship a committed `.env.example` (placeholders); keep
  the real `.env` and credentials gitignored. No hardcoded secrets, none in logs.
- **Clean, simple code, in CLI form.**

**The script contract:** JSON in / JSON out, no LLM. JSON to stdout, a
human-readable summary to stderr, with a `--pretty` flag. Shared helpers go in
`scripts/lib/`.

**`scripts/CLAUDE.md`, the script index.** Every `scripts/` folder carries a
`CLAUDE.md` listing what each script does, its I/O contract, and a usage example.
Update it whenever a script is added. We use `CLAUDE.md` (not `README.md`)
because the harness auto-loads it as context when the agent works in that folder.

## 2. References vs assets

`SKILL.md` is an index, not an encyclopedia: **keep it under 200 lines**. It
points to the rest of the skill's knowledge, split into two folders:

- **`references/`, pure knowledge.** Information the agent reads to *understand*:
  guidelines, domain rules, scenarios. Consumed by reading.
- **`assets/`, knowledge used to produce something.** Templates, data files, any
  artifact that is an *input to an operation* rather than something to read.

The test: read it to learn → reference. A step *uses* it to build output → asset.

## 3. Parallelization and subagents

Map the steps: which are independent, which depend on a previous result.
Independent steps should run in parallel.

**§3.1: Subagents: one task, a clean context.**

- **One task, then return.** A subagent does a single job and hands its result
  back to the parent; it doesn't run the whole process.
- **A subagent exists for judgment, not for running a CLI.** Its reason to exist
  is an output that's hard to get with code: it executes some actions and *makes
  a decision* from its prompt, then passes it back. Don't spin one up just to run
  a script: if a step is pure deterministic execution, call the script directly.
- **It also keeps the parent's context clean**, a real but secondary benefit.
- **Keep agents generic.** Push specifics into the prompt the skill hands it, not
  into the agent definition.
- **The skill orchestrates and decides;** the agents are generic workers.
- **Skills have no subagents of their own; plugins do** (their `agents/*.md`). A
  process that needs subagents wants to be a plugin. A plugin groups multiple
  skills and provides the generic agents they coordinate.

A subagent can't pause to ask the user: it runs to a proposal and returns; the
main conversation is where you stop, show the user, and wait for approval.

## 4. Preflight: verify the environment first

The **first thing every skill does** is run a preflight: a script in `scripts/`
(`setup.sh` on Unix, `setup.bat` on Windows) that confirms the environment is
ready: required config files exist, Python is installed if used, any
skill-specific tools/env vars are present. If something is missing, it **fails
loudly with exactly what to provide** and the skill stops. Fail fast on a clean,
fixable error beats failing deep in the process.

Document it in **`references/setup.md`** (every skill ships one): what the
preflight verifies, how to satisfy each requirement, and how to run it.

## Checklist

- [ ] `SKILL.md` is under 200 lines and reads as an index.
- [ ] A preflight (`scripts/setup.sh` / `.bat`) runs first and verifies the
      skill's requirements; `references/setup.md` documents it.
- [ ] Every measurable step is a deterministic script, not agent judgment.
- [ ] Scripts have no external dependencies and are commented.
- [ ] No secrets committed; `.env.example` present, real `.env` gitignored.
- [ ] `scripts/CLAUDE.md` lists every script and is up to date.
- [ ] Knowledge is split correctly between `references/` and `assets/`.
- [ ] Independent steps are parallelized; skill-vs-plugin chosen deliberately.
- [ ] Agents are generic and single-task; the skill owns orchestration and the
      final decision.
- [ ] Every subagent earns its place with judgment; none exists only to run a
      script.
