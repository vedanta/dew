#!/usr/bin/env bash
# Discovery: scan (init --from-gitignore and add . land with #20/#21).
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

# A .gitignore with a real candidate and some noise.
printf '.env.local\nnode_modules/\n*.log\n' >"$REPO/.gitignore"
echo "TOKEN=abc" >"$REPO/.env.local"
mkdir -p "$REPO/node_modules/dep"
echo "junk" >"$REPO/node_modules/dep/index.js"
echo "log" >"$REPO/debug.log"

run_dew scan
assert_success
assert_contains "Candidates:"
assert_contains ".env.local"
assert_contains "Skipped"
assert_contains "node_modules/"

# Noise must not be offered as a candidate: it should only appear under Skipped.
candidates_section="${LAST_OUTPUT%%Skipped*}"
case "$candidates_section" in
*node_modules*) fail "node_modules offered as a candidate" ;;
*debug.log*) fail "debug.log offered as a candidate" ;;
esac

echo "  scan ok"
