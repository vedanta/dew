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

echo "  pack ok"
