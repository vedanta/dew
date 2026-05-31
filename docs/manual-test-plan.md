# dew — Manual Test Plan

A copy-pasteable walkthrough that exercises every command against a throwaway
dummy repo. Expected outputs below are from a real run; exact paths/keys will
differ on your machine.

> **Isolation:** every step sets `DEW_HOME` to a temp directory so the test
> **never touches your real `~/.dew/`** (identity, images, config). If you omit
> `DEW_HOME`, `dew` uses `~/.dew` and `dew keygen` would create a real identity.

## 0. Prerequisites

Build the binary and set up an isolated sandbox:

```bash
# from the repo root
go build -o /tmp/dew .

# sandbox: isolated dew home, a fake "remote", and a dummy source repo
export DEW_HOME="$(mktemp -d)/dewhome"
export STORE="$(mktemp -d)/remote"
export REPO="$(mktemp -d)/acme-api"
alias dew=/tmp/dew     # so the commands below read naturally

mkdir -p "$REPO/certs" "$REPO/config" && cd "$REPO"
```

Create a realistic `.gitignore`, some ignored local files, noise, and one
tracked source file:

```bash
cat > .gitignore <<'EOF'
.env.local
*.local
certs/
config/secrets.json
node_modules/
*.log
EOF

echo 'API_KEY=dev-123'   > .env.local
echo 'db.local-stuff'    > app.local
echo '---CERT---'        > certs/dev.pem
echo '{"token":"abc"}'   > config/secrets.json
echo 'package main'      > main.go                 # tracked (not ignored)
mkdir -p node_modules/x && echo junk > node_modules/x/i.js
echo 'noise'             > debug.log
```

---

## 1. Identity

```bash
dew keygen
```
Expected:
```
Created identity
  Private key: <DEW_HOME>/identity.age.key
  Public key:  age1...
```

```bash
dew key status
```
Expected: `Identity: Present` with the same public key.

✅ **Check:** `ls -l "$DEW_HOME"` shows `identity.age.key` (mode `0600` on Unix),
`identity.age.pub`, and an `images/` dir.

```bash
dew keygen        # second time
```
Expected: errors with `identity: already exists ...`, exit code `1`. ✅ refuses to overwrite.

---

## 2. Discovery & manifest setup

```bash
dew init
```
Expected: `Created .dew/manifest.yaml (project "acme-api")`.

```bash
dew scan
```
Expected — candidates suggested, noise excluded:
```
Candidates:
  .env.local
  app.local
  certs/dev.pem
  config/secrets.json

Skipped (noise):
  debug.log
  node_modules/
```
✅ **Check:** `node_modules/` and `debug.log` are under *Skipped*, never *Candidates*; tracked source `main.go` appears in neither.

---

## 3. Add / list / remove

```bash
dew add .env.local certs/dev.pem config/secrets.json    # explicit paths
dew add . --yes                                          # discovered candidates (non-interactive)
dew list
```
Expected `list`:
```
Project: acme-api

Tracked:
  .env.local
  certs/dev.pem
  config/secrets.json
  app.local
```
✅ **Check:** `add . --yes` picked up only `app.local` (the remaining candidate) — **not** `node_modules`/`debug.log`, and not `main.go`.

Interactive form (answer `y`/`n` at the prompts):
```bash
dew remove app.local      # drop it again
dew add .                 # prompts: Add app.local? [Y/n]
```

```bash
dew remove not-tracked-file     # removing something absent
```
Expected: `not tracked: not-tracked-file`, exit `0` (clean no-op).

---

## 4. Pack & health

```bash
dew status            # before packing
```
Expected includes:
```
Image:     Missing (run 'dew pack')
Tracked:   4
Hydration: Healthy
```

```bash
dew pack
```
Expected: `Packed 4 tracked path(s) → <DEW_HOME>/images/acme-api.dew.age`.

✅ **Check — the image is encrypted, not plaintext:**
```bash
grep -a 'API_KEY' "$DEW_HOME/images/acme-api.dew.age" && echo "LEAK!" || echo "ok: secret not in image"
file "$DEW_HOME/images/acme-api.dew.age"   # → data
```

```bash
dew doctor
```
Expected ends with `Repository fully hydrated.`

---

## 5. Sync (push) to a local destination

```bash
dew sync          # no destination configured yet
```
Expected: errors with `no destination configured — set sync.destination in ~/.dew/config.yaml`, exit `1`.

```bash
printf 'sync:\n  destination: %s\n' "$STORE" > "$DEW_HOME/config.yaml"
dew sync
```
Expected: `Pushed acme-api.dew.age → <STORE>`.

✅ **Check:** `ls "$STORE"` shows `acme-api.dew.age`.

> A **remote** destination works the same way — set `destination: nas:/volume1/dew`
> (a `host:path`) and `dew sync` shells out to `scp`, using your `~/.ssh/config`.
> If `scp` isn't installed you get a clear "required tool scp not found" message.

---

## 6. Fresh-clone hydrate (the defining workflow)

Simulate a new machine: the repo and identity are present, but the local image
and local files are gone.

```bash
rm "$DEW_HOME/images/acme-api.dew.age"     # no local image
rm .env.local                              # working tree is "dry"

dew status        # → Hydration: Incomplete: 1 file(s) missing ...
dew sync pull     # → Pulled acme-api.dew.age from <STORE>
dew restore       # → Restored: N written, ...
cat .env.local    # → API_KEY=dev-123   (restored byte-for-byte)
dew doctor        # → Repository fully hydrated.
```
✅ **Check:** restored files match the originals exactly.

---

## 7. Non-destructive restore & --force

```bash
echo 'API_KEY=LOCAL-EDIT' > .env.local     # diverge from the image
dew restore
```
Expected — conflict reported, file untouched, exit `1`:
```
Restored: 0 written, 0 unchanged, 0 overwritten, 1 conflict(s)
  conflict: .env.local (differs from image; left unchanged)
Error: restore: 1 file(s) differ from the image; re-run with --force to overwrite
```
✅ **Check:** `cat .env.local` still shows `API_KEY=LOCAL-EDIT` — **no silent data loss.**

```bash
dew restore --force
cat .env.local        # → API_KEY=dev-123   (now overwritten)
```

---

## 8. Security & safety checks

**Path safety — can't add outside the repo:**
```bash
dew add ../escape
```
Expected: `add: path "../escape" is outside the repository`, exit `1`.

**Per-manifest deny — a `deny:` rule keeps matches out of candidates:**
```bash
printf 'deny:\n  - "*.secret"\n' >> .dew/manifest.yaml
printf '*.secret\n' >> .gitignore
echo s > api.secret
dew scan          # api.secret appears under "Skipped", never "Candidates"
```

**Sync refuses keys** (defensive): the sync layer rejects `*.key` / `identity.age*`
paths, so the private key can never be pushed.

---

## 9. Diagnosis (doctor) matrix

Point `doctor` at fresh/broken states (each in its own temp dew home / repo):

```bash
# no identity yet
cd "$(mktemp -d)" && DEW_HOME="$(mktemp -d)" /tmp/dew doctor
#   → Problem: No identity found.  →  Run 'dew keygen' ...
```
Other states to spot-check by removing pieces: no manifest → `dew init`;
empty allow-list → `dew add`; no image → `dew pack`; tracked file missing →
`dew restore`.

---

## 10. Cleanup

```bash
unalias dew 2>/dev/null
unset DEW_HOME STORE REPO
rm -rf /tmp/dew
# the mktemp -d sandboxes under $TMPDIR can be removed too
```

---

## Pass criteria summary

- [ ] keygen creates a `0600` identity and refuses to overwrite
- [ ] scan suggests git-ignored files, excludes noise; `add .` never sweeps in noise or tracked source
- [ ] pack produces an encrypted image (secret not present in plaintext)
- [ ] sync push/pull round-trips the image (local destination)
- [ ] restore reproduces files byte-for-byte after a "fresh clone"
- [ ] restore is non-destructive: diverged files are conflicts, preserved without `--force`
- [ ] `add ../escape` and other outside-repo paths are rejected
- [ ] per-manifest `deny:` keeps matches out of candidates
- [ ] doctor reports the right problem + next action for each broken state, and "fully hydrated" when healthy
