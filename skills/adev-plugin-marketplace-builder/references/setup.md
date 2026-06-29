# Setup — adev-plugin-marketplace-builder

What this skill needs before it runs. The preflight `scripts/setup.sh`
(`scripts/setup.bat` on Windows) checks it automatically — run it first.

## Requirements

- **The `adev` CLI on PATH** (required). It scaffolds the marketplace this skill
  fills in: `adev new plugin-marketplace <name>`. Repo:
  https://github.com/gverdugo-dev/agentic-developer
- **git** (recommended). Hosting a marketplace on a git remote (GitHub, GitLab)
  is the standard distribution path. The preflight warns if git is missing but
  does not block — you only need it at the hosting step.

No API keys or `.env` to configure: this skill is guidance plus a template.

## Run the check

```bash
bash scripts/setup.sh      # Unix/macOS
scripts\setup.bat          # Windows
```

Fix anything it reports before using the skill.
