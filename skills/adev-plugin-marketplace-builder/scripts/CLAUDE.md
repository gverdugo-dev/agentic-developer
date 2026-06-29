# scripts/ script index

Deterministic helpers for adev-plugin-marketplace-builder. Standard tools only,
no external dependencies. Update this index whenever a script is added.

## `setup.sh` / `setup.bat`: preflight check

Verifies the skill's requirements before any work: that the `adev` CLI is on
PATH (required) and warns if `git` is missing (recommended for hosting). Prints
a human-readable result to stderr and exits non-zero if a required tool is
missing.

```bash
bash scripts/setup.sh      # Unix/macOS
scripts\setup.bat          # Windows
```
