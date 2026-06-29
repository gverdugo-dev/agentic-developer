# Opinionated skill-creation philosophy

This is the house style behind adev: how we think a skill (or a plugin) should
be built. adev scaffolds the folders; this document explains the reasoning that
should fill them. The four sections below are the load-bearing ideas — read them
in order.

## 0. Skills as operators / no-code automations

Today a skill is the easiest way to automate something. With very little you can
build something powerful: a single skill can encode a complete, ordered process
— the steps, the decisions, the quality checks — and run it end to end. Treat a
skill as an *operator*: a small, self-contained worker that owns one job and does
it the same way every time.

## 1. Determinism whenever possible

Prefer code over judgment. Whenever a step can be made deterministic, write a
small script for it instead of asking the agent to do it by reasoning. An agent
guessing its way through an external service is fragile; a tiny CLI that calls
that service is repeatable.

**Before writing the script**, validate the prerequisites: does the user have an
API key, a token, credentials? Only build the integration once you know it can
actually run.

Scripts live in the skill's `scripts/` folder and follow these rules, always:

- **No external dependencies.** Standard library only. `pip install` should not
  be required to run a script. If something must be signed or parsed, reach for
  what the OS already provides before adding a dependency.
- **Documented with comments.** Explain intent, not syntax. Name your constants
  — no magic numbers (a threshold of `30` should say *why* it is 30).
- **Configuration via `.env`** when (and only when) it is needed. Load it from a
  small shared helper, not by scattering `os.environ` reads everywhere.
- **Never commit secrets.** Keys and credentials stay out of the repo. Ship a
  committed `.env.example` (placeholders, no real values) and keep the real
  `.env` and any credential files gitignored. Audit for security mistakes — no
  hardcoded secrets, no secrets in logs.
- **Clean, simple code, in CLI form.** Each script is a small command-line tool.

### The script contract

Make scripts composable and predictable. The convention that works:

- **JSON in / JSON out, no LLM.** A script fetches or transforms data and applies
  measurable rules. It does not reason; it computes.
- **JSON to stdout, a human-readable summary to stderr.** This keeps pipes clean
  while still giving a person something to read. Offer a `--pretty` flag to
  indent the JSON.
- **Shared helpers in `scripts/lib/`.** Common concerns (env loading, HTTP,
  parsing) live in one place and are reused across scripts.

### `scripts/CLAUDE.md` — the script library index

Every `scripts/` folder carries a `CLAUDE.md` that acts as the library index: a
short table of contents describing what each script does, its input/output
contract, and a usage example. **Every time a new script is added, this index is
updated** with its explanation. This is what lets the agent pick the right tool
without reading every file.

We use `CLAUDE.md` (not `README.md`) on purpose: the harness auto-loads a
`CLAUDE.md` as context whenever the agent works inside that folder, so the script
index is always in view without an explicit read.

> A worked example of this whole section — deterministic CLIs, a `lib/` folder,
> a committed `.env.example`, and a per-folder script index — lives in the
> `seo-analysis` skill under `tmp/.claude/plugins/billingham-marketing/skills/`.

## 2. References vs assets

The `SKILL.md` file is an index, not an encyclopedia. **It must stay under 200
lines.** It points to the rest of the skill's knowledge, which lives in two
clearly separated folders:

- **`references/` — pure knowledge.** Information the agent reads to *understand*:
  guidelines, domain rules, scenarios, data-source notes. It is consumed by
  reading.
- **`assets/` — knowledge that gets used to produce something.** Templates (an
  HTML or markdown skeleton), data files (a JSON the script consumes), or any
  artifact that is an *input to an operation* rather than something to be read.

The test: if the agent reads it to learn, it is a reference. If a step *uses* it
to build an output, it is an asset.

## 3. Parallelization whenever possible

When designing a skill, map its steps: which ones are independent and which
depend on a previous result. Independent steps should run in parallel. Whenever a
skill has work that doesn't depend on other work, it can fan that work out
instead of doing it serially.

### 3.1 Subagents: one task, a clean context

Parallel (and context-isolating) work is launched as subagents. What defines a
subagent:

- **One task, then return.** A subagent does a single job and hands its result
  back to the parent. It does not run the whole process — it contributes one
  piece of it.
- **A subagent exists for judgment, not for running a CLI.** Its reason to exist
  is to produce an output that would be hard to get with code alone: it executes
  some actions and then *makes a decision* based on what its prompt tells it, and
  passes that decision back to the parent. **Don't spin up a subagent just to run
  a script** — if a step is pure deterministic execution, the skill calls the
  script directly.
- **It also keeps the parent's context clean.** A subagent works in its own
  context and returns only its conclusion, so the intermediate noise never
  reaches the parent. That isolation is a real benefit — but it's secondary to
  the judgment the subagent provides.
- **Agents can run scripts — as part of deciding.** An agent may run
  deterministic CLIs to gather or check data, but its value is the decision it
  reaches on top of them, not the execution itself.
- **Keep agents generic.** The more reusable an agent is, the better. Push the
  specifics into the prompt the skill hands it, not into the agent definition.

Division of labor:

- **The skill orchestrates and decides.** The skill runs the orchestration —
  sequencing phases, fanning out independent work, and making the final decision.
  The agents are generic workers; the skill is the conductor.
- **Skills have no subagents of their own; plugins do.** A plugin ships
  `agents/*.md` definitions that its skills launch (via the native subagent
  integration). So a process that needs subagents wants to be a plugin.
- **A plugin is a package that groups multiple skills** — a company department, a
  single larger process — and provides the generic agents those skills
  coordinate.

A subagent runs until it produces its result and returns; it cannot pause to ask
the user. Design phases accordingly: a subagent runs to a proposal, and the main
conversation is where you stop, show the user, and wait for approval before
feeding the next phase.

## Checklist

Before calling a skill done, verify:

- [ ] `SKILL.md` is under 200 lines and reads as an index.
- [ ] Every measurable step is a deterministic script, not agent judgment.
- [ ] Scripts have no external dependencies and are commented.
- [ ] No secrets committed; `.env.example` present, real `.env` gitignored.
- [ ] `scripts/CLAUDE.md` lists every script and is up to date.
- [ ] Knowledge is split correctly between `references/` and `assets/`.
- [ ] Independent steps are parallelized; skill-vs-plugin chosen deliberately.
- [ ] Agents are generic and single-task; the skill owns orchestration and the
      final decision.
- [ ] Every subagent earns its place with judgment — none exists only to run a
      script (call the script directly instead).
