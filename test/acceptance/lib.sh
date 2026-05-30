# shellcheck shell=bash
# Shared helpers for dew acceptance tests.
#
# Each test script sources this file, calls setup_sandbox (which isolates $HOME
# and gives a throwaway repo), then drives the built `dew` binary via run_dew
# and checks results with the assert_* helpers. Tests never touch the real
# ~/.dew/.
set -euo pipefail

# Resolve the binary up front so a missing build fails loudly rather than
# silently passing. Override with DEW=/path/to/dew.
DEW="${DEW:-dew}"
if ! command -v "$DEW" >/dev/null 2>&1; then
	echo "acceptance: dew binary not found on PATH (DEW=$DEW)" >&2
	exit 1
fi
DEW="$(command -v "$DEW")"

# setup_sandbox creates an isolated HOME and a throwaway repo dir, arranges
# cleanup on exit, and cds into the repo. Sets SANDBOX, HOME and REPO.
setup_sandbox() {
	SANDBOX="$(mktemp -d 2>/dev/null || mktemp -d -t dew-acc)"
	export SANDBOX
	export HOME="$SANDBOX/home"
	REPO="$SANDBOX/repo"
	mkdir -p "$HOME" "$REPO"
	trap teardown_sandbox EXIT
	cd "$REPO"
}

teardown_sandbox() {
	[ -n "${SANDBOX:-}" ] && rm -rf "$SANDBOX"
}

# run_dew runs the binary, capturing combined output into LAST_OUTPUT and the
# exit code into LAST_STATUS without aborting the test on non-zero exit.
run_dew() {
	set +e
	LAST_OUTPUT="$("$DEW" "$@" 2>&1)"
	LAST_STATUS=$?
	set -e
}

fail() {
	echo "  ASSERT FAILED: $*" >&2
	exit 1
}

assert_success() {
	[ "${LAST_STATUS:-1}" -eq 0 ] || fail "expected success, got status ${LAST_STATUS:-?}: ${LAST_OUTPUT:-}"
}

assert_failure() {
	[ "${LAST_STATUS:-0}" -ne 0 ] || fail "expected failure, got success: ${LAST_OUTPUT:-}"
}

assert_contains() {
	case "${LAST_OUTPUT:-}" in
	*"$1"*) : ;;
	*) fail "output does not contain '$1': ${LAST_OUTPUT:-}" ;;
	esac
}

assert_file_exists() {
	[ -e "$1" ] || fail "expected file to exist: $1"
}
