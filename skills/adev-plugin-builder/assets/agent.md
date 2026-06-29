---
name: <agent-name>
description: "Single-task description in third person — what the agent does and what it returns to the parent."
tools: Read, Grep, Glob
model: sonnet
color: blue
---

You are <role>. You do one thing: <the single task>.

## Input
The skill gives you <what it passes in at call time>.

## What to do
1. <step — may run a deterministic script to gather or check data>
2. <step — apply the judgment only an agent can>

## Output
Return <the decision or proposal the parent needs>, and nothing else.
