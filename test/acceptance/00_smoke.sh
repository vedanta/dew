#!/usr/bin/env bash
# Smoke test: the binary runs, exposes its command surface, and stubs behave
# predictably. Real per-command acceptance tests land with their phases.
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

# --help succeeds and lists core commands.
run_dew --help
assert_success
assert_contains "pack"
assert_contains "restore"
assert_contains "sync"

# --version succeeds and prints a version line.
run_dew --version
assert_success
assert_contains "dew version"

# The version subcommand prints richer build info.
run_dew version
assert_success
assert_contains "commit:"
assert_contains "go:"

# Unknown commands are rejected.
run_dew definitely-not-a-real-command
assert_failure

# The sandbox really is isolated: no manifest created yet.
[ ! -e "$REPO/.dew/manifest.yaml" ] || fail "unexpected manifest in fresh sandbox"

echo "  smoke ok"
