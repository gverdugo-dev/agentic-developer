#!/usr/bin/env bash
# Preflight for adev-plugin-builder: verify the skill's requirements exist
# before any work. Exits non-zero with a clear message if something is missing.
set -euo pipefail

missing=0
fail() {
	echo "ERROR: $1" >&2
	missing=1
}

# The adev CLI scaffolds the plugin folders this skill fills in.
if ! command -v adev >/dev/null 2>&1; then
	fail "'adev' CLI not found on PATH. Install it: https://github.com/gverdugo-dev/agentic-developer"
fi

if [ "$missing" -ne 0 ]; then
	echo "Preflight failed: fix the items above, then re-run." >&2
	exit 1
fi

echo "Preflight OK: adev is available." >&2
