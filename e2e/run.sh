#!/bin/sh
# run.sh drives the pty e2e suite: it builds adev, then runs every e2e/*.exp
# flow in a real pseudo-terminal against a fresh fixture home per test.
#
#   e2e/run.sh              run every test
#   e2e/run.sh view_nav     run a single test by name
#   E2E_VERBOSE=1 e2e/run.sh   also print the raw pty stream
#
# Requirements: the Go toolchain and the `expect` binary (preinstalled on
# macOS; `apt install expect` on Debian/Ubuntu). On failure a test keeps its
# fixture dir, including the ADEV_TUI_LOG key trace, for debugging.
set -eu

cd "$(dirname "$0")/.."

if ! command -v expect >/dev/null 2>&1; then
    echo "e2e: the 'expect' binary is required (macOS ships it; apt install expect)" >&2
    exit 1
fi

go build -o bin/adev ./cmd/adev
ADEV_BIN="$(pwd)/bin/adev"
export ADEV_BIN

filter="${1:-}"
pass=0
fail=0
total=0

for test in e2e/*.exp; do
    name="$(basename "$test" .exp)"
    if [ -n "$filter" ] && [ "$name" != "$filter" ]; then
        continue
    fi
    total=$((total + 1))

    WORK="$(mktemp -d "${TMPDIR:-/tmp}/adev-e2e.XXXXXX")"
    sh e2e/fixture.sh "$WORK"
    export E2E_WORK="$WORK"
    export E2E_HOME="$WORK/home"
    export E2E_ROOT="$WORK/alpha-root"
    export E2E_BETA="$WORK/beta-root"

    echo "=== $name"
    if expect -f "$test"; then
        echo "--- PASS: $name"
        pass=$((pass + 1))
        rm -rf "$WORK"
    else
        echo "--- FAIL: $name (fixtures kept at $WORK)"
        fail=$((fail + 1))
    fi
done

if [ "$total" -eq 0 ]; then
    echo "e2e: no test matches \"$filter\"" >&2
    exit 1
fi

echo "e2e: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
