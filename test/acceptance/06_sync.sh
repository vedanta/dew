#!/usr/bin/env bash
# Sync: push (pull + full hydrate land with #27). Uses a local directory as the
# destination, so no SSH/scp is involved.
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

run_dew keygen
assert_success
run_dew init
assert_success
echo "TOKEN=abc" >"$REPO/.env.local"
run_dew add .env.local
assert_success
run_dew pack
assert_success

# Without a configured destination, sync fails clearly.
run_dew sync
assert_failure
assert_contains "no destination"

# Configure a local destination and push.
store="$SANDBOX/remote"
printf 'sync:\n  destination: %s\n' "$store" >"$HOME/.dew/config.yaml"

run_dew sync
assert_success
assert_contains "Pushed"
assert_file_exists "$store/repo.dew.age"

echo "  sync push ok"
