#!/usr/bin/env bash
# Pack (and, with #17, restore round-trip). Sandbox isolates $HOME so the
# identity and images live under the throwaway home.
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

# Set up identity + manifest + a tracked file.
run_dew keygen
assert_success
run_dew init
assert_success
echo "TOKEN=abc" >"$REPO/.env.local"
run_dew add .env.local
assert_success

# pack produces an encrypted image under ~/.dew/images.
run_dew pack
assert_success
assert_contains "Packed"
assert_file_exists "$HOME/.dew/images/repo.dew.age"

# The image must be encrypted, not plaintext: the secret must not appear in it.
if grep -aq "TOKEN=abc" "$HOME/.dew/images/repo.dew.age"; then
	fail "secret appears in plaintext inside the image"
fi

# Round-trip: simulate a fresh clone (local file gone), then restore.
rm "$REPO/.env.local"
run_dew restore
assert_success
assert_contains "1 written"
assert_file_exists "$REPO/.env.local"
[ "$(cat "$REPO/.env.local")" = "TOKEN=abc" ] || fail "restored content does not match original"

# Restoring again is a no-op (identical).
run_dew restore
assert_success
assert_contains "1 unchanged"

# Non-destructive: a diverged local file is NOT overwritten without --force.
printf 'TOKEN=local-edit\n' >"$REPO/.env.local"
run_dew restore
assert_failure
assert_contains "conflict"
[ "$(cat "$REPO/.env.local")" = "TOKEN=local-edit" ] || fail "restore clobbered local changes without --force"

# --force overwrites with the image content.
run_dew restore --force
assert_success
[ "$(cat "$REPO/.env.local")" = "TOKEN=abc" ] || fail "--force did not restore image content"

echo "  pack/restore round-trip ok"
