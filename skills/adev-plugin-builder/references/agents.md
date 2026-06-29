# Writing a generic agent

An agent (`agents/<name>.md`) is a generic, single-task worker a skill launches
to get a decision back. This is the craft that makes a plugin worth building.

## Contents

- Frontmatter
- The prompt body
- The rules that make an agent good
- Example

## Frontmatter

```yaml
---
name: keyword-expert
description: "Single-task description in third person — what the agent does and what it returns."
tools: Read, Grep, Glob, Bash
model: sonnet
color: green
---
```

- **`name`** — lowercase, hyphens; the skill launches it as `<plugin>:<name>`.
- **`description`** — third person, one task, and what it returns to the parent.
- **`tools`** — the minimum set the task needs. Fewer tools = sharper, safer
  agent. Grant `Bash` only if it must run scripts.
- **`model`** — match the task: `haiku` for cheap/mechanical, `sonnet` for most
  work, `opus` for hard reasoning.
- **`color`** — optional, for display.

## The prompt body

The body is the agent's system prompt. Keep it **generic**: describe the role and
the single job, not the specifics of one invocation — the skill passes those at
call time. Give it a clear output contract so the parent gets exactly what it
needs.

```markdown
You are <role>. You do one thing: <the single task>.

## Input
The skill gives you <what it passes in>.

## What to do
1. <step — may run a script to gather or check data>
2. <step — apply judgment>

## Output
Return <the decision / proposal the parent needs>, and nothing else.
```

## The rules that make an agent good

- **One task, then return.** It contributes one piece; it does not run the whole
  process.
- **It exists for judgment, not to run a CLI.** If a step is pure deterministic
  execution, the skill calls the script directly — don't wrap a CLI in an agent.
  An agent may *run* scripts, but its value is the decision it reaches on top.
- **Generic and reusable.** No invocation-specific details baked in; the skill
  supplies those. The same agent should serve many calls.
- **Clean context in, clean answer out.** It works in its own context and returns
  only its conclusion, so it never pollutes the parent's context.
- **It can't pause.** It runs to a proposal and returns; the skill handles any
  user approval.

## Example

```markdown
---
name: competitor-analyst
description: "Analyzes one competitor URL's on-page SEO and returns a ranked list of content gaps versus our page. Returns a JSON proposal of gaps."
tools: Read, Bash
model: sonnet
color: red
---

You are an SEO competitor analyst. You do one thing: compare a competitor page
to ours and propose the content gaps worth closing.

## Input
The skill gives you our page's audit JSON and the competitor URL.

## What to do
1. Run the on-page audit script on the competitor URL to get its structured data.
2. Compare against our page; judge which gaps are real opportunities, not noise.

## Output
Return a JSON list of gaps, each with a short rationale, ranked by impact. Nothing
else.
```
