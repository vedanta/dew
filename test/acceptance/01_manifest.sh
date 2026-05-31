#!/usr/bin/env bash
# Manifest lifecycle: init + add (remove/list land with #7/#8).
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

# init creates a manifest at .dew/manifest.yaml.
run_dew init
assert_success
assert_contains "Created"
assert_file_exists "$REPO/.dew/manifest.yaml"
grep -q "^project:" "$REPO/.dew/manifest.yaml" || fail "manifest missing project field"
grep -q "^version:" "$REPO/.dew/manifest.yaml" || fail "manifest missing version field"

# add appends a real file to the allow-list.
echo "TOKEN=abc" >"$REPO/.env.local"
run_dew add .env.local
assert_success
assert_contains "added .env.local"
grep -q ".env.local" "$REPO/.dew/manifest.yaml" || fail "allow-list missing .env.local"

# add is idempotent.
run_dew add .env.local
assert_success
assert_contains "already tracked"

# add accepts a directory path too.
mkdir -p "$REPO/certs"
echo "pem" >"$REPO/certs/dev.pem"
run_dew add certs/dev.pem
assert_success
assert_contains "added certs/dev.pem"

# add rejects paths outside the repository.
run_dew add ../escape
assert_failure
assert_contains "outside the repository"

# list reflects what has been added.
run_dew list
assert_success
assert_contains "Project:"
assert_contains ".env.local"
assert_contains "certs/dev.pem"

# remove drops a tracked path.
run_dew remove .env.local
assert_success
assert_contains "removed .env.local"
grep -q "env.local" "$REPO/.dew/manifest.yaml" && fail "allow-list still has .env.local after remove"

# removing an untracked path is a clean no-op.
run_dew remove never-added
assert_success
assert_contains "not tracked"

# list now reflects the removal.
run_dew list
assert_success
assert_contains "certs/dev.pem"
case "$LAST_OUTPUT" in
*".env.local"*) fail "list still shows removed .env.local" ;;
esac

# init refuses to clobber an existing manifest.
run_dew init
assert_failure
assert_contains "already exists"

# --from-gitignore and --project are exposed.
run_dew init --help
assert_success
assert_contains "from-gitignore"
assert_contains "project"

# --project sets a custom name (in a separate source dir).
alt="$SANDBOX/other-src"
mkdir -p "$alt"
cd "$alt"
run_dew init --project custom-name
assert_success
grep -q "^project: custom-name$" "$alt/.dew/manifest.yaml" || fail "project name not set by --project"
grep -q "^image: custom-name.dew.age$" "$alt/.dew/manifest.yaml" || fail "image not derived from --project"
# An invalid project name is rejected.
mkdir -p "$SANDBOX/bad-src"
cd "$SANDBOX/bad-src"
run_dew init --project "../escape"
assert_failure
assert_contains "project name"
cd "$REPO"

echo "  manifest ok"
