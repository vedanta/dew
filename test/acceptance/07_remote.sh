#!/usr/bin/env bash
# dew remote: set / show / unset the sync destination, and dew status reflecting
# it. Uses a local path as the destination — no SSH involved.
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

run_dew keygen
assert_success
run_dew init
assert_success

# Nothing configured yet.
run_dew remote
assert_success
assert_contains "No remote configured"

# Setting an empty destination is rejected.
run_dew remote set ""
assert_failure

# Set, then show.
dest="$SANDBOX/remote"
run_dew remote set "$dest"
assert_success
assert_contains "$dest"

run_dew remote
assert_success
assert_contains "$dest"

# dew status reflects the configured destination.
run_dew status
assert_success
assert_contains "$dest"

# sync now knows where to go (push succeeds without hand-editing config).
echo "TOKEN=abc" >"$REPO/.env.local"
run_dew add .env.local
assert_success
run_dew pack
assert_success
run_dew sync
assert_success
assert_contains "Pushed"

# Unset clears it, and sync then points at 'dew remote set'.
run_dew remote unset
assert_success
assert_contains "cleared"

run_dew remote
assert_success
assert_contains "No remote configured"

run_dew sync
assert_failure
assert_contains "dew remote set"

echo "  remote set/show/unset + status + sync wiring ok"
