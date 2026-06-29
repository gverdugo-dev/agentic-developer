#!/usr/bin/env bash
# TEMPLATE — preflight for <skill-name>.
# Drop this into the new skill's scripts/ folder and customize the checks for
# what the skill actually needs. Delete the examples that don't apply.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
missing=0
fail() {
	echo "ERROR: $1" >&2
	missing=1
}

# --- Examples: keep what applies, delete the rest --------------------------

# Python, if the skill's scripts use it:
# command -v python3 >/dev/null 2>&1 || fail "python3 not found — install Python 3"

# A config file with credentials (never committed):
# [ -f "$here/.env" ] || fail "scripts/.env missing — copy scripts/.env.example and fill it"

# A required CLI tool on PATH:
# command -v <tool> >/dev/null 2>&1 || fail "<tool> not found on PATH"

# A required environment variable:
# [ -n "${SOME_API_KEY:-}" ] || fail "SOME_API_KEY not set"

# ---------------------------------------------------------------------------

if [ "$missing" -ne 0 ]; then
	echo "Preflight failed: fix the items above, then re-run." >&2
	exit 1
fi

echo "Preflight OK." >&2
