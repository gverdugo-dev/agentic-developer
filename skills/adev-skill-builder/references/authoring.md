# Authoring SKILL.md

How to write the `SKILL.md` file so the agent discovers and uses the skill well.

## Contents

- Frontmatter
- The body
- Progressive disclosure
- Style rules

## Frontmatter

Two required fields, in YAML at the top of the file:

```yaml
---
name: my-skill
description: "What the skill does and when to use it, in third person."
---
```

**`name`** — lowercase letters, numbers, and hyphens only; max 64 characters;
must not contain the reserved words `anthropic` or `claude`. Match the skill's
folder name. Noun phrases (`pdf-processing`) or gerunds (`processing-pdfs`) both
read well; be consistent across a collection.

**`description`** — the single most important field: the agent reads it to decide
whether to trigger the skill. Rules:

- **Write in third person.** Good: "Generates X…". Avoid: "I can help you…",
  "You can use this to…".
- **Say both what it does and when to use it**, with concrete triggers and key
  terms. Include the phrases a user would actually say.
- Max 1024 characters; keep it tight.

Good: `"Extracts text and tables from PDF files, fills forms, merges documents.
Use when working with PDFs or when the user mentions PDFs, forms, or extraction."`

Bad: `"Helps with documents"` — too vague to match on.

## The body

`SKILL.md` is an **index, not an encyclopedia** — keep it **under 200 lines**.
Assume the agent is already smart: add only what it doesn't know. Don't explain
general concepts; state the skill's specific job, workflow, and where the detail
lives.

A reliable shape:

1. One-line statement of what the skill does.
2. The preflight call (run `scripts/setup.sh` first).
3. The workflow — clear, numbered steps; for multi-step processes, give a
   checklist the agent copies and ticks off.
4. Pointers to `references/` and `assets/` for the detail.

For fragile or exact steps, be prescriptive ("run exactly this"). For open-ended
steps where many approaches work, give direction and trust the agent.

## Progressive disclosure

Move detail out of `SKILL.md` into separate files the agent loads only when
needed. Two rules:

- **Keep references one level deep.** Every reference links directly from
  `SKILL.md`. Avoid files that point to other files that point to the content —
  the agent may only preview deep chains and miss information.
- **Give long reference files (>100 lines) a table of contents** at the top, so
  the agent sees the full scope even on a partial read.

Name files for their content (`commands.md`, `data-sources.md`), not `doc1.md`.

## Style rules

- **Forward slashes** in every path, even for Windows (`scripts/setup.sh`).
- **Consistent terminology** — pick one term per concept and keep it.
- **No time-sensitive information** ("after August…"); put deprecated guidance in
  an "old patterns" section instead.
- **Provide a default, not a menu.** One recommended approach with an escape
  hatch beats listing five options.
- **Concrete examples over abstract description** — show input → output.
