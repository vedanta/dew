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

# Full hydrate capstone: simulate a fresh clone / new machine that has the repo
# and identity but no local image and no local files, then pull + restore.
rm "$HOME/.dew/images/repo.dew.age"
rm "$REPO/.env.local"

run_dew sync pull
assert_success
assert_contains "Pulled"
assert_file_exists "$HOME/.dew/images/repo.dew.age"

# 'dew hydrate' is an alias for restore.
run_dew hydrate
assert_success
[ "$(cat "$REPO/.env.local")" = "TOKEN=abc" ] || fail "hydrate did not restore content"

run_dew doctor
assert_success
assert_contains "fully hydrated"

echo "  sync push + pull + hydrate (restore alias) ok"
