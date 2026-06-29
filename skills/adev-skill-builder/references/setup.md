# Setup — adev-skill-builder

What this skill needs before it runs. The preflight `scripts/setup.sh`
(`scripts/setup.bat` on Windows) checks it automatically — run it first.

## Requirements

- **The `adev` CLI on PATH.** It scaffolds the skill folders this skill then
  fills in: `adev new skill <name>`. Repo:
  https://github.com/gverdugo-dev/agentic-developer

This skill has no other external dependencies — it is guidance plus templates,
not a service integration, so there are no API keys or `.env` to configure.

## Run the check

```bash
bash scripts/setup.sh      # Unix/macOS
scripts\setup.bat          # Windows
```

Fix anything it reports before using the skill.
