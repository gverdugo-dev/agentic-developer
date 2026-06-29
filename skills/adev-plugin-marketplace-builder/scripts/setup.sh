#!/usr/bin/env bash
# Preflight for adev-plugin-marketplace-builder: verify the skill's requirements
# exist before any work. Exits non-zero if a required tool is missing; git is a
# soft recommendation (warning only) since it's needed only at the hosting step.
set -euo pipefail

missing=0
fail() {
	echo "ERROR: $1" >&2
	missing=1
}

# The adev CLI scaffolds the marketplace this skill fills in (required).
if ! command -v adev >/dev/null 2>&1; then
	fail "'adev' CLI not found on PATH. Install it: https://github.com/gverdugo-dev/agentic-developer"
fi

# git is recommended for hosting the marketplace remotely (not required here).
if ! command -v git >/dev/null 2>&1; then
	echo "WARNING: git not found. You'll need it to host the marketplace on a remote (GitHub/GitLab)." >&2
fi

if [ "$missing" -ne 0 ]; then
	echo "Preflight failed: fix the items above, then re-run." >&2
	exit 1
fi

echo "Preflight OK: adev is available." >&2
