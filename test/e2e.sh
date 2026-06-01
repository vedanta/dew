#!/usr/bin/env bash
#
# End-to-end test for dew: a realistic two-machine workflow plus the edge cases.
# Unlike the per-command acceptance scripts, this simulates two separate
# machines (separate DEW_HOMEs), a shared "remote", and a manual key transfer.
#
# Usage:  test/e2e.sh            (builds dew from source, runs everything)
#         DEW=/path/to/dew test/e2e.sh   (use an existing binary)
#
# check conditions are single-quoted on purpose (passed to eval), and the
# captured vars are used inside them — so silence the resulting false positives.
# shellcheck disable=SC2016,SC2034,SC2001
set -u

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ROOT="$(mktemp -d)"
M1="$ROOT/machine1-home"   # ~/.dew on the source machine
M2="$ROOT/machine2-home"   # ~/.dew on the new machine
STORE="$ROOT/remote"       # shared sync destination (e.g. a NAS dir)
SRC="$ROOT/src/acme-api"   # the source repo

PASS=0
FAIL=0
say()  { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }
ok()   { printf '  \033[32mPASS\033[0m %s\n' "$1"; PASS=$((PASS + 1)); }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$1"; FAIL=$((FAIL + 1)); }
check() { if eval "$2"; then ok "$1"; else bad "$1 [cond: $2]"; fi; }

cleanup() { rm -rf "$ROOT"; }
trap cleanup EXIT

# Build dew unless a binary was provided.
DEW="${DEW:-$ROOT/dew}"
if [ "$DEW" = "$ROOT/dew" ]; then
	say "Build"
	( cd "$REPO_ROOT" && go build -o "$DEW" . ) || { echo "build failed"; exit 1; }
	echo "  built $DEW"
fi

mkdir -p "$SRC/certs" "$SRC/config"

############################################################
say "MACHINE 1 — author sets up the repo"
export DEW_HOME="$M1"
cd "$SRC" || exit 1

cat >.gitignore <<'EOF'
.env.local
*.local
certs/
config/secrets.json
node_modules/
*.log
EOF
echo 'API_KEY=prod-secret-123' >.env.local
echo 'cache.local'             >app.local
echo '---DEV CERT---'          >certs/dev.pem
echo '{"db":"s3cr3t"}'         >config/secrets.json
echo 'package main'            >main.go
mkdir -p node_modules/lib && echo junk >node_modules/lib/i.js
echo 'log line'                >debug.log

"$DEW" keygen >/dev/null
PUB1="$("$DEW" key status | awk '/Public key/{print $3}')"
check "keygen created identity" '[ -f "$M1/identity.age.key" ]'

"$DEW" init >/dev/null
check "manifest created" '[ -f "$SRC/.dew/manifest.yaml" ]'
check "manifest has a committed id" 'grep -q "^id:" "$SRC/.dew/manifest.yaml"'

# Per-manifest deny + a denied file inside an allow-listed dir.
printf 'deny:\n  - "*.tmp"\n' >>"$SRC/.dew/manifest.yaml"
echo 'keep-me' >config/app.conf
echo 'scratch' >config/build.tmp

say "scan classifies candidates vs noise"
SCAN="$("$DEW" scan)"
echo "$SCAN" | sed 's/^/    /'
CAND="${SCAN%%Skipped*}"
check "scan suggests .env.local" 'echo "$CAND" | grep -q ".env.local"'
check "scan hides node_modules"  '! echo "$CAND" | grep -q "node_modules"'
check "scan hides debug.log"     '! echo "$CAND" | grep -q "debug.log"'

say "rules shows the layered deny config"
RULES="$("$DEW" rules)"
echo "$RULES" | sed 's/^/    /'
check "rules shows built-in deny" 'echo "$RULES" | grep -q "Deny — built-in"'
check "rules shows repo deny *.tmp" 'echo "$RULES" | grep -q "\*.tmp"'

say "add (explicit + discovered) and list"
"$DEW" add .env.local certs/dev.pem config/secrets.json config >/dev/null
"$DEW" add . --yes >/dev/null
LIST="$("$DEW" list)"
echo "$LIST" | sed 's/^/    /'
check "list shows .env.local"    'echo "$LIST" | grep -q ".env.local"'
check "list shows certs/dev.pem" 'echo "$LIST" | grep -q "certs/dev.pem"'

say "pack --dry-run writes nothing"
DRY="$("$DEW" pack --dry-run)"
echo "$DRY" | sed 's/^/    /'
check "dry-run announces itself" 'echo "$DRY" | grep -q "Dry run"'
check "dry-run wrote no image"   '[ ! -f "$M1/images/acme-api.dew.age" ]'

say "pack"
"$DEW" pack
IMG="$M1/images/acme-api.dew.age"
check "image produced"                          '[ -f "$IMG" ]'
check "image is encrypted (no plaintext secret)" '! grep -aq "prod-secret-123" "$IMG"'
check "ownership marker written"                 '[ -f "$IMG.id" ]'

say "deny: build.tmp excluded, app.conf included (verify via restore into scratch)"
SCRATCH="$ROOT/inspect"
mkdir -p "$SCRATCH/.dew"
cp "$SRC/.dew/manifest.yaml" "$SCRATCH/.dew/manifest.yaml"
( cd "$SCRATCH" && "$DEW" restore >/dev/null 2>&1 )
check "deny excluded build.tmp"  '[ ! -f "$SCRATCH/config/build.tmp" ]'
check "allowed app.conf packed"  '[ -f "$SCRATCH/config/app.conf" ]'

say "doctor + status + images"
"$DEW" status | sed 's/^/    /'
DOC="$("$DEW" doctor)"
check "doctor reports fully hydrated" 'echo "$DOC" | grep -q "fully hydrated"'
IMAGES="$("$DEW" images)"
echo "$IMAGES" | sed 's/^/    /'
check "images lists acme-api"         'echo "$IMAGES" | grep -q "acme-api.dew.age"'

say "global deny (~/.dew/config.yaml) hides a candidate"
printf 'sync:\n  destination: %s\ndeny:\n  - "*.bak"\n' "$STORE" >"$M1/config.yaml"
echo 'b' >"$SRC/zz.bak"
printf 'zz.bak\n' >>"$SRC/.gitignore"
GCAND="$("$DEW" scan)"; GCAND="${GCAND%%Skipped*}"
check "global deny keeps zz.bak out of candidates" '! echo "$GCAND" | grep -q "zz.bak"'

say "sync push"
"$DEW" sync
check "image pushed to remote" '[ -f "$STORE/acme-api.dew.age" ]'

############################################################
say "init --project gives an independent name"
PROJ="$ROOT/src/weird-folder"
mkdir -p "$PROJ"
( cd "$PROJ" && "$DEW" init --project billing-svc >/dev/null )
check "project name set by --project" 'grep -q "^project: billing-svc$" "$PROJ/.dew/manifest.yaml"'
check "image name derived from --project" 'grep -q "^image: billing-svc.dew.age$" "$PROJ/.dew/manifest.yaml"'

say "cross-repo collision guard"
OTHER="$ROOT/src/other"
mkdir -p "$OTHER"
( cd "$OTHER" && "$DEW" init --project acme-api >/dev/null 2>&1; echo x >t.txt; "$DEW" add t.txt >/dev/null )
if ( cd "$OTHER" && "$DEW" pack >/dev/null 2>&1 ); then bad "cross-repo pack should be refused"; else ok "cross-repo pack refused"; fi
if ( cd "$OTHER" && "$DEW" pack --force >/dev/null 2>&1 ); then ok "cross-repo pack --force succeeds"; else bad "--force pack failed"; fi
# restore machine 1's real image ownership.
( cd "$SRC" && "$DEW" pack --force >/dev/null )

############################################################
say "MACHINE 2 — fresh clone (committed files only, NO local secrets)"
DST="$ROOT/clone/acme-api"
mkdir -p "$DST/.dew"
cp "$SRC/.gitignore" "$DST/.gitignore"
cp "$SRC/main.go" "$DST/main.go"
cp "$SRC/.dew/manifest.yaml" "$DST/.dew/manifest.yaml"
cd "$DST" || exit 1
export DEW_HOME="$M2"
check "fresh clone has no .env.local" '[ ! -f "$DST/.env.local" ]'

say "wrong identity can fetch but not decrypt"
"$DEW" keygen >/dev/null
printf 'sync:\n  destination: %s\n' "$STORE" >"$M2/config.yaml"
"$DEW" sync pull >/dev/null
DOC2="$("$DEW" doctor)"
echo "$DOC2" | sed 's/^/    /'
check "doctor flags undecryptable image" 'echo "$DOC2" | grep -q "cannot be decrypted"'

say "transfer the correct identity"
cp "$M1/identity.age.key" "$M2/identity.age.key"
cp "$M1/identity.age.pub" "$M2/identity.age.pub"
PUB2="$("$DEW" key status | awk '/Public key/{print $3}')"
check "public keys now match" '[ "$PUB1" = "$PUB2" ]'

say "hydrate: sync pull -> dew hydrate (restore alias)"
"$DEW" sync pull >/dev/null
"$DEW" hydrate | sed 's/^/    /'
check "restored .env.local matches"    '[ "$(cat "$DST/.env.local")" = "API_KEY=prod-secret-123" ]'
check "restored certs/dev.pem matches" '[ "$(cat "$DST/certs/dev.pem")" = "---DEV CERT---" ]'
check "restored config/secrets.json"   '[ "$(cat "$DST/config/secrets.json")" = "{\"db\":\"s3cr3t\"}" ]'
DOC3="$("$DEW" doctor)"
check "doctor: fully hydrated"          'echo "$DOC3" | grep -q "fully hydrated"'

say "non-destructive restore + --dry-run + --force"
echo 'API_KEY=LOCAL-EDIT' >"$DST/.env.local"
DRYR="$("$DEW" restore --dry-run)"
echo "$DRYR" | sed 's/^/    /'
check "dry-run reports a conflict"      'echo "$DRYR" | grep -q "conflict"'
check "dry-run changed nothing"         '[ "$(cat "$DST/.env.local")" = "API_KEY=LOCAL-EDIT" ]'
if "$DEW" restore >/dev/null 2>&1; then bad "restore should refuse to clobber"; else ok "restore refuses to clobber local edit"; fi
check "local edit still preserved"      '[ "$(cat "$DST/.env.local")" = "API_KEY=LOCAL-EDIT" ]'
"$DEW" restore --force >/dev/null
check "--force overwrites with image"   '[ "$(cat "$DST/.env.local")" = "API_KEY=prod-secret-123" ]'

############################################################
say "RESULTS"
printf '  passed=%d failed=%d\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
