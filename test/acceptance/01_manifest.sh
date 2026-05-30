#!/usr/bin/env bash
# Manifest lifecycle: init (and, as later phases land, add/remove/list).
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

# init creates a manifest at .dew/manifest.yaml.
run_dew init
assert_success
assert_contains "Created"
assert_file_exists "$REPO/.dew/manifest.yaml"

# The created manifest is valid YAML naming this repo as the project.
grep -q "^project:" "$REPO/.dew/manifest.yaml" || fail "manifest missing project field"
grep -q "^version:" "$REPO/.dew/manifest.yaml" || fail "manifest missing version field"

# init refuses to clobber an existing manifest.
run_dew init
assert_failure
assert_contains "already exists"

# --from-gitignore is accepted (discovery itself lands in Phase 4).
run_dew init --from-gitignore --help
assert_success
assert_contains "from-gitignore"

echo "  manifest ok"
