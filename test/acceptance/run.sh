#!/usr/bin/env bash
# Acceptance test runner for dew.
#
# Builds nothing itself — expects the `dew` binary on PATH (the CI job and the
# Makefile `acceptance` target build it first). Runs every NN_*.sh test in
# order and fails fast on the first failing script.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"

shopt -s nullglob
tests=("$here"/[0-9]*.sh)
shopt -u nullglob

if [ "${#tests[@]}" -eq 0 ]; then
	echo "no acceptance tests found in $here"
	exit 0
fi

for t in "${tests[@]}"; do
	name="$(basename "$t")"
	echo "=== $name ==="
	if bash "$t"; then
		echo "PASS: $name"
	else
		echo "FAIL: $name" >&2
		exit 1
	fi
done

echo "----"
echo "all ${#tests[@]} acceptance test(s) passed"
