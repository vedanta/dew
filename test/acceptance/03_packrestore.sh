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

# pack --dry-run previews the file list and writes nothing.
run_dew pack --dry-run
assert_success
assert_contains "Dry run"
assert_contains ".env.local"
[ ! -e "$HOME/.dew/images/repo.dew.age" ] || fail "dry-run wrote an image"

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
# restore --dry-run previews without writing.
run_dew restore --dry-run
assert_success
assert_contains "Dry run"
[ ! -e "$REPO/.env.local" ] || fail "dry-run restore wrote a file"
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

# Pack-time ownership: a different repo with the same image name is refused.
other="$SANDBOX/other-repo"
mkdir -p "$other"
cd "$other"
run_dew init --project repo        # same image name as the first repo (basename "repo")
assert_success
echo "OTHER" >other.txt
run_dew add other.txt
assert_success
run_dew pack
assert_failure
assert_contains "different repo"
run_dew pack --force               # explicit override succeeds
assert_success
cd "$REPO"

# pack --all packs the LOCAL half (what Git doesn't carry), deny-filtered,
# .git/.dew excluded, allow-list untouched.
allrepo="$SANDBOX/all-repo"
mkdir -p "$allrepo/src" "$allrepo/node_modules/pkg"
cd "$allrepo"
run_dew init
assert_success
echo "SECRET=1" >.env.local
echo "hello" >src/main.txt
echo "wip" >notes.md
echo "noise" >node_modules/pkg/x.js
git -C "$allrepo" init -q
git -C "$allrepo" add src/main.txt
git -C "$allrepo" -c user.email=t@t -c user.name=t commit -qm seed

run_dew pack --all --dry-run
assert_success
assert_contains ".env.local"
assert_contains "notes.md"
case "$LAST_OUTPUT" in
*src/main.txt*) fail "pack --all must not sweep tracked files: $LAST_OUTPUT" ;;
esac
case "$LAST_OUTPUT" in
*node_modules* | *.git/config* | *.dew/manifest*) fail "pack --all dry-run swept in noise or structural dirs: $LAST_OUTPUT" ;;
esac

run_dew pack --all
assert_success
assert_contains "local file"
assert_file_exists "$HOME/.dew/images/all-repo.dew.age"

# Outside a git repo, --all refuses with guidance.
plaindir="$SANDBOX/plain-dir"
mkdir -p "$plaindir"
cd "$plaindir"
run_dew init
assert_success
echo "x" >file.txt
run_dew pack --all
assert_failure
assert_contains "not a git repository"
cd "$allrepo"

# The allow-list stayed empty: a normal pack still refuses.
run_dew pack
assert_failure
assert_contains "nothing to pack"
cd "$REPO"

# restore --image hydrates from an explicit file — no manifest required.
carried="$SANDBOX/carried.dew.age"
cp "$HOME/.dew/images/all-repo.dew.age" "$carried"
fresh="$SANDBOX/fresh-clone"
mkdir -p "$fresh"
cd "$fresh"
run_dew restore --image "$carried"
assert_success
assert_file_exists "$fresh/notes.md"
[ "$(cat "$fresh/.env.local")" = "SECRET=1" ] || fail "restore --image did not carry .env.local content"
[ ! -e "$fresh/src/main.txt" ] || fail "tracked src/main.txt leaked into the --all image"
run_dew restore --image "$SANDBOX/no-such.dew.age"
assert_failure
assert_contains "--image"
cd "$REPO"

# Explicitly added files beat the deny-list (and add says so); dir-swept
# noise stays filtered.
echo keepme >"$REPO/keep.log"
run_dew add keep.log
assert_success
assert_contains "the explicit add overrides it"
run_dew pack --dry-run
assert_success
assert_contains "keep.log"
mkdir -p "$REPO/node_modules"
run_dew add node_modules
assert_success
assert_contains "deny-listed"
run_dew remove node_modules          # keep the allow-list clean for later scripts
assert_success

echo "  pack/restore round-trip + ownership guard + pack --all + restore --image + deny interplay ok"
