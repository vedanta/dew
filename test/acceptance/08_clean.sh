#!/usr/bin/env bash
# Teardown: `dew clean` removes the manifest + image, and `dew images rm` deletes
# images by project name. Sandbox isolates $HOME so removals never touch the real
# ~/.dew/.
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
assert_file_exists "$HOME/.dew/images/repo.dew.age"

# clean --image-only -y drops the image but keeps the manifest.
run_dew clean --image-only -y
assert_success
assert_contains "Removed the image"
[ ! -e "$HOME/.dew/images/repo.dew.age" ] || fail "image should have been removed"
assert_file_exists "$REPO/.dew/manifest.yaml"

# Re-pack, then a non-interactive clean without --force/--yes is refused (no
# confirmation available) and removes nothing.
run_dew pack
assert_success
printf 'n\n' | "$DEW" clean >/dev/null 2>&1 && fail "declined clean should exit non-zero" || true
assert_file_exists "$REPO/.dew/manifest.yaml"
assert_file_exists "$HOME/.dew/images/repo.dew.age"

# clean --force removes both and drops the now-empty .dew dir.
run_dew clean --force
assert_success
assert_contains "no longer manages this repo"
[ ! -e "$REPO/.dew" ] || fail ".dew dir should be gone"
[ ! -e "$HOME/.dew/images/repo.dew.age" ] || fail "image should be gone"

# images rm garbage-collects by project name, including the .id marker.
run_dew init
assert_success
echo "TOKEN=xyz" >"$REPO/.env.local"
run_dew add .env.local
assert_success
run_dew pack
assert_success
assert_file_exists "$HOME/.dew/images/repo.dew.age.id"

run_dew images rm repo --yes
assert_success
assert_contains "removed"
[ ! -e "$HOME/.dew/images/repo.dew.age" ] || fail "images rm left the image"
[ ! -e "$HOME/.dew/images/repo.dew.age.id" ] || fail "images rm left the .id marker"

# Unknown project is a harmless no-op; traversal is rejected.
run_dew images rm ghost --yes
assert_success
assert_contains "no image for"
run_dew images rm ../escape --yes
assert_failure
assert_contains "invalid project name"

echo "  clean + images rm ok"
