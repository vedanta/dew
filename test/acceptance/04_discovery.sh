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

# init --from-gitignore seeds the allow-list from discovered candidates only.
run_dew init --from-gitignore
assert_success
assert_contains "Seeded"
run_dew list
assert_success
assert_contains ".env.local"
case "$LAST_OUTPUT" in
*node_modules*) fail "node_modules was seeded into the manifest" ;;
*debug.log*) fail "debug.log was seeded into the manifest" ;;
esac

# 'dew add . --yes' adds discovered candidates non-interactively. Add a new
# ignored file, then confirm add . picks it up but never sweeps in noise.
echo "v" >"$REPO/.env.dev"
printf '.env.dev\n' >>"$REPO/.gitignore"
run_dew add . --yes
assert_success
run_dew list
assert_success
assert_contains ".env.dev"
case "$LAST_OUTPUT" in
*node_modules*) fail "add . swept in node_modules" ;;
*debug.log*) fail "add . swept in debug.log" ;;
esac

# Per-manifest deny: a deny: rule removes a matching path from candidates.
printf 'deny:\n  - "*.secret"\n' >>"$REPO/.dew/manifest.yaml"
printf '*.secret\n' >>"$REPO/.gitignore"
echo "k" >"$REPO/api.secret"
run_dew scan
assert_success
candidates_section="${LAST_OUTPUT%%Skipped*}"
case "$candidates_section" in
*api.secret*) fail "deny-listed api.secret offered as a candidate" ;;
esac

# dew rules shows the effective allow/deny by layer.
run_dew rules
assert_success
assert_contains "Deny — built-in"
assert_contains "node_modules/"
assert_contains "Deny — repo"
assert_contains "*.secret"

# Global deny (~/.dew/config.yaml) keeps a match out of candidates.
mkdir -p "$HOME/.dew"
printf 'deny:\n  - "*.scratch"\n' >"$HOME/.dew/config.yaml"
printf '*.scratch\n' >>"$REPO/.gitignore"
echo s >"$REPO/tmp.scratch"
run_dew rules
assert_success
assert_contains "Deny — global"
assert_contains "*.scratch"
run_dew scan
assert_success
candidates_section="${LAST_OUTPUT%%Skipped*}"
case "$candidates_section" in
*tmp.scratch*) fail "global deny did not keep tmp.scratch out of candidates" ;;
esac

echo "  scan + init --from-gitignore + add . + deny (repo + global) + rules ok"
