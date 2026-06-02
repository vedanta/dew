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
deliberately ignores the **private, per-developer** state needed to actually run
a clone: `.env.local`, dev certificates, `docker-compose.override.yml`, private
fixtures, local config.

dew manages exactly that ignored state. It packages an allow-listed set of files
into a single **encrypted image** per repo and can sync that image to a remote,
so after a fresh `git clone` you can **hydrate** the repo back to a working
state:

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

dew is a single self-contained Go binary. Until release binaries are published,
build from source (**Go 1.26+**):

```bash
git clone https://github.com/vedanta/dew && cd dew
go build -o dew .          # or: make build
sudo mv dew /usr/local/bin/   # optional: put it on your PATH
dew --version
```

The only external runtime dependency is `scp`, and only when you sync to a
remote `host:path` destination — local/mounted destinations need nothing.
Encryption and compression are built in (no external `age`/`zstd`).

## 3. Concepts

**Two-location model.** dew keeps state in two places:

- **In the repo (committed to Git):** `.dew/manifest.yaml` — the contract. It
  declares the project name, image name, the **allow-list** of files to manage,
  and an optional **deny-list**. It never contains secrets, file contents, or
  keys.
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
sync destination, and **your identity** (the private key — dew never syncs it;
copy `~/.dew/identity.age.*` over yourself, e.g. via a password manager or
secure transfer).

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
   `build/`, `target/`, `.venv/`, `__pycache__/`, `.DS_Store`, `*.log`.
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
     - ".next/"
   ```

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

## 9. Syncing

Sync copies the encrypted image to/from a single destination. Set the
destination once with **`dew remote`** (no need to hand-edit config):

```bash
dew remote set nas:/volume1/dew   # remote (scp) — or a local/mounted path
dew remote                        # show the current destination
dew remote unset                  # clear it
```

The destination lives in `~/.dew/config.yaml` and is shared across all your
repos. Then:

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
- **`dew images`** — a global inventory of every image dew manages (project,
  size, last-packed time, owning repo id). Runs from anywhere.

## 11. Identity & keys

- **`dew keygen`** creates the one global identity; it refuses to overwrite.
- **`dew key status`** shows whether it's present and its public key.
- The **private key is never synced or committed.** To use dew on another
  machine, copy `~/.dew/identity.age.key` (and `.pub`) there yourself. Treat it
  like any other private key. Key backup/rotation is out of scope for now — keep
  a secure copy.
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
