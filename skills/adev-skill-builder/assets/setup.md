# Setup — <skill-name>

TEMPLATE — drop this into the new skill's `references/` folder and fill in the
blanks. Describes what the skill needs before it can run; the preflight
`scripts/setup.sh` (`scripts/setup.bat` on Windows) checks these automatically.

## Requirements

- <e.g. Python 3, if the scripts use it>
- <e.g. scripts/.env with API_KEY=... — copy from scripts/.env.example>
- <e.g. a CLI tool on PATH>

## How to satisfy them

1. <step to install or configure each requirement>
2. <...>

## Run the check

```bash
bash scripts/setup.sh      # Unix/macOS
scripts\setup.bat          # Windows
```

Fix anything it reports before using the skill.
