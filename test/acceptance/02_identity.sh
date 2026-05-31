#!/usr/bin/env bash
# Identity lifecycle: keygen (key status lands with #11). The sandbox sets an
# isolated $HOME, so keygen writes to $HOME/.dew, never the real identity.
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

# Before keygen, status reports no identity.
run_dew key status
assert_success
assert_contains "Not found"

# keygen creates the global identity.
run_dew keygen
assert_success
assert_contains "Public key"
assert_contains "age1"
assert_file_exists "$HOME/.dew/identity.age.key"
assert_file_exists "$HOME/.dew/identity.age.pub"

# images/ directory is created alongside the identity.
[ -d "$HOME/.dew/images" ] || fail "keygen did not create ~/.dew/images"

# After keygen, status reports the identity and its public key.
run_dew key status
assert_success
assert_contains "Present"
assert_contains "age1"

# keygen refuses to overwrite an existing identity.
run_dew keygen
assert_failure
assert_contains "already exists"

echo "  identity ok"
