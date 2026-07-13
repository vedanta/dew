# dew — User Manual

A practical guide to using **dew**, the local-first CLI that manages the private
repository state Git intentionally ignores. For a terse per-command listing see
the [command reference](COMMANDS.md); for the design rationale see
[`design.md`](design.md).

## Contents
1. [What dew is](#1-what-dew-is)
2. [Installing](#2-installing)
3. [Concepts](#3-concepts)
4. [Getting started (first repo)](#4-getting-started-first-repo)
5. [Hydrating a fresh clone (new machine)](#5-hydrating-a-fresh-clone-new-machine)
6. [Choosing what to track](#6-choosing-what-to-track)
7. [The deny-list (three layers)](#7-the-deny-list-three-layers)
8. [Packing and restoring](#8-packing-and-restoring)
9. [Syncing](#9-syncing)
10. [Health & inventory](#10-health--inventory)
11. [Identity & keys](#11-identity--keys)
12. [Security model](#12-security-model)
13. [Troubleshooting](#13-troubleshooting)
14. [What dew is not](#14-what-dew-is-not)

---

## 1. What dew is

Git tracks the **shared** state of a project — source, docs, manifests. It
deliberately ignores the **private, per-developer** context needed to actually
run a clone: `.env.local` and secrets, dev certificates,
`docker-compose.override.yml`, private fixtures, machine-specific config, and the
local notes you keep out of Git.

dew manages exactly that local context — the part Git can't hold. (Shared docs
still belong in Git; dew is for what shouldn't.) It packages an allow-listed set
of files into a single **encrypted image** per repo and can sync that image to a
remote, so after a fresh `git clone` you can **hydrate** the repo back to a
working state:

```bash
git clone <repo> && cd <repo>
dew sync pull   # fetch the encrypted image
dew restore     # extract the local files back into the working tree
```

Git gives you the code. dew gives you the missing local context — it
**complements Git** and never touches your tracked source.

Every command has a self-contained `dew <command> --help` (what it does, why,
what happens, what to run next), and `dew --help` groups the commands by purpose.
When in doubt, `dew doctor` diagnoses the repo and names the next command to run.

## 2. Installing

dew is a single self-contained Go binary.

```bash
# Homebrew (macOS/Linux)
brew install --cask vedanta/dew/dew

# or a binary from https://github.com/vedanta/dew/releases/latest
# or with Go 1.26+:
go install github.com/vedanta/dew@latest
```

**Updating:** `dew upgrade` fetches the latest release (or an exact tag with
`--version v0.6.0`), verifies it against the release checksums, and swaps the
binary in place — `--check` previews without changing anything. Homebrew
installs update with `brew upgrade --cask dew` instead (dew detects this and
tells you).

The only external runtime dependencies are OpenSSH's `scp` (syncing to a remote
`host:path`) and `ssh` (`dew remote test`/`images` against a remote) — both ship
together, and only remote destinations need them; local/mounted destinations
need nothing. Encryption and compression are built in (no external `age`/`zstd`).

## 3. Concepts

**Two-location model.** dew keeps state in two places:

- **In the repo (committed to Git):** `.dew/manifest.yaml` — the contract. It
  declares the project name, image name, the **allow-list** of files to manage,
  and an optional **deny-list**. It never contains secrets, file contents, or
  keys. `dew init` also drops a short `.dew/README.md` explaining the directory
  to anyone browsing the repo (GitHub renders it in the folder listing).
- **In your home (never committed):** `~/.dew/` holds `config.yaml`, the global
  age keypair (`identity.age.key` / `.pub`), and `images/<project>.dew.age` —
  the encrypted shadow images.

```
~/.dew/
├── config.yaml              # sync destination, global deny-list
├── identity.age.key         # private key (0600) — NEVER synced/committed
├── identity.age.pub
└── images/
    └── <project>.dew.age     # encrypted image, one per repo
```

**One identity, one image per repo.** There is a single global keypair shared
across all your repos, and one encrypted image per repo (keyed by project name).

**Hydration.** A fresh clone is "dry" — it has the code but none of the local
context. `dew restore` settles the missing files back onto it. `dew doctor`
reports `Repository fully hydrated.` when everything is in place.

**`DEW_HOME`.** Set this environment variable to use a directory other than
`~/.dew` (handy for testing or isolating identities).

## 4. Getting started (first repo)

One-time, per machine:

```bash
dew keygen            # creates ~/.dew identity (refuses to overwrite)
dew key status        # confirm: Identity: Present + your public key
```

In a repo:

```bash
cd my-app
dew init                      # creates .dew/manifest.yaml (project = "my-app")
# or: dew init --project billing-svc      (name independent of the folder)
# or: dew init --from-gitignore           (seed the allow-list from .gitignore)

dew scan                      # see suggested candidates (noise filtered out)
dew add .env.local certs/     # add specific paths
dew add .                     # or interactively add discovered candidates
dew list                      # review what's tracked

dew pack                      # build ~/.dew/images/my-app.dew.age
git add .dew/manifest.yaml && git commit -m "Add dew manifest" && git push
```

Then point dew at a sync destination and push the image (see
[Syncing](#9-syncing)):

```bash
dew remote set nas:/volume1/dew   # local path or scp-style host:path
dew sync
```

## 5. Hydrating a fresh clone (new machine)

On a second machine, you need three things: the repo (from Git), the configured
sync destination, and **your identity**. dew never *syncs* the private key, but
it gives you one explicit command to provision it over SSH — run it from a
machine that already has the identity:

```bash
dew key push you@newmachine     # from a machine that HAS the identity, push it over
# ── or, run this ON the new machine to pull it from one that has it ──
dew key pull you@oldmachine
```

(Or move the key over yourself — password manager, secure copy. Only
`~/.dew/identity.age.key` is needed: decryption uses the private key alone, and
dew derives the public key from it, so the `.pub` file is optional. Either way,
**don't** run `dew keygen` on the new machine: that mints a different identity
that can't decrypt your images.)

```bash
git clone <repo> && cd my-app
dew key status     # make sure your identity is present here
dew sync pull      # fetch the encrypted image
dew restore        # extract the local files
dew doctor         # → Repository fully hydrated.
```

If `dew doctor` reports the image *cannot be decrypted*, the identity on this
machine doesn't match the one that packed it — transfer the correct key.

## 6. Choosing what to track

The allow-list is **authoritative**: `pack` only ever includes paths the
manifest lists — never "everything ignored."

- **`dew scan`** reads `.gitignore` and the working tree to *suggest* candidates,
  with noise filtered out. It only suggests; you opt in.
- **`dew add <path>`** adds specific files or directories (adding a directory
  includes its files, minus deny-listed noise).
- **`dew add .`** adds the *discovered candidates* (prompting `Y/n`, or `-y` to
  accept all) — **not** every file in the repo.
- **`dew remove <path>`** drops an entry; **`dew list`** shows the current set.

Paths outside the repo (e.g. `../secret`) are rejected.

## 7. The deny-list (three layers)

The deny-list guarantees noise stays out, even when you add a whole directory.
There are three layers, all visible via **`dew rules`**:

1. **Built-in** — universal noise shipped with dew: `node_modules/`, `dist/`,
   `build/`, `target/`, `.venv/`, `__pycache__/`, `.next/`, `.nuxt/`,
   `coverage/`, `.cache/`, `.turbo/`, `.parcel-cache/`, `Pods/`,
   `DerivedData/`, `.gradle/`, `.cxx/`, `.expo/`, `.DS_Store`, `*.log`,
   `*.tsbuildinfo`.
2. **Global** — your per-user noise, applied to *every* repo. Add a `deny:` list
   to `~/.dew/config.yaml`:
   ```yaml
   deny:
     - "*.swp"
     - ".idea/"
   ```
3. **Repo** — project-specific, in `.dew/manifest.yaml`:
   ```yaml
   deny:
     - "*.tmp"
     - "fixtures-huge/"
   ```

**What belongs in each layer.** The built-in list is deliberately kept minimal
and universal — only paths that are regenerated in *essentially every* project
that contains them (`node_modules/`, `Pods/`, `.gradle/`, …). It intentionally
does **not** try to know every framework's generated directories, because a path
that's throwaway output in one project is hand-written source in another. The
clearest example: an [Expo](https://docs.expo.dev/workflow/continuous-native-generation/)
app regenerates `ios/` and `android/` with `expo prebuild`, so they're local
noise there — but in a bare React Native app those same directories *are* the
source. dew can't tell the two apart (Git shows both as "not tracked"), so the
call is yours: exclude them per-repo when they're regenerable —

```yaml
# .dew/manifest.yaml — this repo generates its native dirs
deny:
  - "packages/mobile/ios/"
  - "packages/mobile/android/"
```

This keeps the built-in list small and predictable, and puts project knowledge
where it lives: in the repo. Use `dew rules` to see the effective result, and
`dew pack --all --dry-run` to check what an image would contain before packing.

**Overriding a rule (`!` negation).** Deny lines use gitignore syntax, including
negation — and layers are evaluated in order **built-in → global → repo**, with
the last matching rule winning. So a repo manifest can rescue anything a
broader layer denies:

```yaml
deny:
  - "!keep.log"    # un-deny one file caught by the built-in *.log
  - "!.next/"      # un-deny a built-in directory rule, for this repo only
```

Two caveats, both matching git's own semantics: a negation can't re-include
files under a directory the walk has pruned (`!Pods/keep.txt` alone does
nothing — `!Pods/` is the unit of rescue), and within one list a later line
beats an earlier one.

**Explicit file adds also win.** A file you `dew add` *by name* is always
packed, even if the deny-list matches it — naming an exact file is stronger
intent than any pattern (`add` prints a note when this happens). Directory
entries stay deny-filtered, and `add` warns if the directory itself is
deny-listed.

Deny patterns use `.gitignore` syntax. They apply to discovery (`scan`,
`add .`, `init --from-gitignore`) and to `pack` (so an allow-listed directory
never sweeps in denied files).

## 8. Packing and restoring

**Pack** builds the image: `tar → zstd → age encrypt → ~/.dew/images/<project>.dew.age`.

```bash
dew pack
dew pack --dry-run    # preview the file list + sizes; write nothing
```

> **You declare files once, then just pack.** `dew add` records a path in the
> committed manifest — a one-time declaration per file. After that, `dew pack`
> re-packages the *current contents* of everything already listed; you don't
> re-`add`. So the steady-state flow is just `dew pack && dew sync` after you
> edit a tracked file. (`dew init --from-gitignore` can seed the allow-list at
> setup, so you can skip the explicit `add` on the first run.)

**`pack --all` is the one-shot exception** to allow-list-authoritative packing:
it sweeps every **local** file — everything Git doesn't carry, ignored and
not-yet-committed alike — into the image, ignoring the allow-list for that
run. It asks `git ls-files` where the boundary is, so it needs Git and a git
repo. The manifest is untouched (your declared set stays as-is), the deny-list
still keeps generated noise out, and `.git/` and `.dew/` are never included.
Git carries the shared half of the repo; `--all` carries everything else:

```bash
dew pack --all --dry-run   # preview the full file list first
dew pack --all
```

dew binds each image to the repo that created it. If a *different* repo would
overwrite an image of the same name, `pack` refuses:

```
pack: my-app.dew.age was created by a different repo; use 'dew init --project <name>'
for a unique name, or --force to overwrite it
```

**Restore** is the reverse, and it is **non-destructive**. It stages files to a
temp directory, then for each file:

- not present locally → **written**,
- identical → **skipped**,
- **differs from the image → reported as a conflict and left untouched** (because
  dew has no version history, it never silently overwrites your local changes).

```bash
dew restore
dew restore --dry-run    # preview written / unchanged / conflict, change nothing
dew restore --force      # overwrite conflicting files with the image
dew hydrate              # same as restore (its own command)
```

A plain `dew restore` with unresolved conflicts exits non-zero so you notice; a
`--dry-run` always exits 0.

By default restore reads this repo's image from `~/.dew/images`. **`--image`**
restores from an explicit `.dew.age` file instead — one copied over by hand,
pulled from a backup, or made with `pack --all` elsewhere. No manifest is
needed (the image defines what it holds); your identity and the same
non-destructive rules apply:

```bash
dew restore --image ~/backups/my-app.dew.age --dry-run
```

**Tearing down.** `dew clean` is the inverse of `init` + `pack`: it removes the
repo's `.dew/` manifest and this repo's image (+ `.id` marker), then drops the
empty `.dew/` directory. It's **local-only** — it never touches your shared
identity key or any copy you've synced to a remote/another machine — and it asks
before deleting unless you pass `--force` (or `-y`/`--yes` to skip just the
prompt). Narrow it with `--image-only` (drop the image, keep the manifest so the
next `dew pack` rebuilds it) or `--manifest-only` (stop managing the repo, keep
the image). Removal is permanent — dew keeps no history — but the image is just a
repack of files still on disk and the manifest is normally committed to Git, so
the usual case is recoverable. To delete a *different* repo's image, or one whose
repo is already gone, use `dew images rm <project>` instead.

```bash
dew clean                  # remove manifest + image (asks first)
dew clean --force          # remove both without confirming
dew clean --image-only     # force a clean re-pack next time
```

## 9. Syncing

Sync copies the encrypted image to/from a single destination. Set the
destination once with **`dew remote`** (no need to hand-edit config):

```bash
dew remote set nas:/volume1/dew   # remote (scp) — or a local/mounted path
dew remote                        # show the current destination
dew remote unset                  # clear it
```

The destination lives in `~/.dew/config.yaml` and is shared across all your
repos. Before relying on it, you can check it's actually usable:

```bash
dew remote test   # reachable? trusted? writable?
```

For a local/mounted path this catches the classic "the NAS isn't mounted" case;
for a remote `host:path` it verifies over `ssh` that the host is reachable, its
key is trusted, and the path is writable (reporting OpenSSH's own verdict). Then:

```bash
dew sync         # push the current repo's image
dew sync pull    # fetch it into ~/.dew/images/
```

**Hybrid transport:**
- **Local / mounted** destinations (e.g. `/Volumes/nas/dew`, a synced folder) use
  a pure-Go copy — no external tools.
- **Remote** `host:path` destinations shell out to **`scp`**, inheriting your
  `~/.ssh/config`, ssh-agent, and `known_hosts`. If `scp` isn't installed you get
  a clear "required tool not found" message — and only for remote destinations.

Sync moves the **encrypted image only** — never the private key. The remote
directory must already exist.

## 10. Health & inventory

- **`dew status`** — a per-repo snapshot: identity, manifest, image, tracked
  count, hydration state, sync config.
- **`dew doctor`** — diagnoses the top problem and tells you the exact next
  command; verifies the image actually decrypts. Reports
  `Repository fully hydrated.` when healthy.
- **`dew images`** — a global inventory of every image dew manages locally
  (project, size, last-packed time, owning repo id). Runs from anywhere.
  **`dew images rm <project>...`** deletes images by name (with their `.id`
  markers) — handy for garbage-collecting an image whose repo is gone.
- **`dew remote images`** — the same view for the *sync destination*: what's
  actually stored there (confirms a push landed, or shows what a new machine can
  pull).

## 11. Identity & keys

- **`dew keygen`** creates the one global identity; it refuses to overwrite.
- **`dew key status`** shows whether it's present and its public key.
- **`dew key push <user@host>`** provisions your identity onto another machine
  over SSH — the one-time bootstrap for a second machine. It verifies the host
  key the normal way, writes the key `0600` under `~/.dew` there, and won't
  overwrite a different identity without `--force`. **`dew key pull <user@host>`**
  is the mirror — run it *on* the new machine to fetch the identity from one that
  already has it (it verifies the download before installing).
- **`dew key devices`** lists where your identity has been sent/received (a local
  `~/.dew/devices.yaml` log, written on both ends of each push/pull). It's a
  best-effort audit log, **not** a registry or revocation tool — manual copies
  aren't recorded, and there's no rotation, so listing a machine doesn't let you
  de-provision it.
- The **private key is never *synced or committed*.** `dew key push` is the one
  explicit, opt-in exception that transmits it — only when you run it, over your
  own SSH access, to a machine you control. (You can still copy the key by hand
  instead — just `~/.dew/identity.age.key`; the `.pub` is optional, since dew
  derives the public key from the private one.) Treat the key like any private
  key; backup/rotation is out of scope — keep a secure copy.
- `DEW_HOME` relocates the whole `~/.dew` directory if you need multiple
  identities or an isolated setup.

## 12. Security model

- **Encryption at rest.** Images are encrypted with [age](https://age-encryption.org)
  (authenticated encryption — tampering or corruption is detected on decrypt).
- **Restore can't escape the repo.** Tar extraction rejects `..` traversal,
  absolute paths, and symlink/hardlink entries (tar-slip / symlink-escape
  defense). An image can never write outside the repo.
- **Restore can't silently destroy data.** Diverged files are conflicts, left
  untouched unless `--force`.
- **Sync never moves keys.** Only encrypted images are transferred; the sync
  layer additionally refuses key-like paths.
- **SSH security is the system's.** Remote sync delegates auth and host-key
  verification to OpenSSH rather than reimplementing it.
- **Image ownership.** Each image is bound to its repo; `pack` won't overwrite an
  image created by a different repo without `--force`.

## 13. Troubleshooting

Run **`dew doctor`** first — it usually tells you exactly what to do. Common
cases:

| Symptom | Cause | Fix |
|---|---|---|
| `no identity found` | Haven't run keygen on this machine | `dew keygen` (or transfer your key) |
| `no manifest found` | Not initialized | `dew init` |
| `Hydration: Incomplete` | Tracked files missing locally | `dew restore` |
| `Image cannot be decrypted` | Wrong identity on this machine | Transfer the matching `~/.dew/identity.age.key` |
| `pack: … created by a different repo` | Two repos share a project name | `dew init --project <unique>`, or `dew pack --force` |
| `restore: N file(s) differ` | Local edits diverge from the image | Inspect with `dew restore --dry-run`; `--force` to overwrite |
| `required tool "scp" not found` | Remote sync without OpenSSH | Install OpenSSH, or use a local/mounted destination |

## 14. What dew is not

dew is **not** a secrets manager, a backup tool, Git LFS, or a cloud sync
service. It is a repo-aware local context manager for files Git intentionally
ignores. The MVP intentionally omits version history, team sharing, multiple
recipients, per-repo keys, and key rotation — see [`design.md`](design.md) for
scope.
