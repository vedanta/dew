#!/usr/bin/env bash
# Health: status (doctor lands with #23).
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/acceptance/lib.sh
. "$here/lib.sh"

setup_sandbox

# Fresh repo: no identity, no manifest.
run_dew status
assert_success
assert_contains "Identity:"
assert_contains "Not found (run 'dew keygen')"
assert_contains "Manifest:"

# After keygen + init + add + pack: healthy.
run_dew keygen
assert_success
run_dew init
assert_success
echo "TOKEN=abc" >"$REPO/.env.local"
run_dew add .env.local
assert_success
run_dew pack
assert_success

run_dew status
assert_success
assert_contains "Present"
assert_contains "Valid"
assert_contains "Healthy"

# dew images lists the global inventory (repo-independent).
run_dew images
assert_success
assert_contains "IMAGE"
assert_contains "repo.dew.age"

# Simulate a fresh clone (file gone, image present): incomplete.
rm "$REPO/.env.local"
run_dew status
assert_success
assert_contains "Incomplete"
assert_contains "dew restore"

# doctor diagnoses the missing file and recommends restore.
run_dew doctor
assert_success
assert_contains "Problem:"
assert_contains "missing"
assert_contains "dew restore"

# After restoring, doctor reports a healthy repo.
run_dew restore
assert_success
run_dew doctor
assert_success
assert_contains "fully hydrated"

echo "  status + doctor ok"
