# dew — Command Reference

Every command, its flags, and behavior. Each command also carries a
self-contained `dew <command> --help` that explains what it does, why, what
happens, and what to run next. For a narrative guide, see the
[user manual](USER-MANUAL.md); the online guide is at <https://vedanta.github.io/dew/>.

`dew --help` groups commands by purpose: **Identity**, **Repository**,
**Image**, **Sync**, and **Health & inventory**. dew complements Git — it never
touches your tracked source; it carries the local context Git is meant to ignore.

## Global

```
dew [command] [flags]
```

| Flag | Description |
|---|---|
| `-h, --help` | Help for any command. |
| `-v, --version` | Print the version. |

### `dew version`
Print the version, commit, build date, Go version, and OS/arch — the same detail behind `-v`, as a subcommand.

```bash
dew version
```

Environment:
- **`DEW_HOME`** — overrides the dew home directory (default `~/.dew`). Useful for testing or isolating multiple identities.

Conventions used below: paths in the manifest are repo-relative; `~/.dew/` holds the global identity, config, and images.

---

## Identity

### `dew keygen`
Create the one global age identity (`~/.dew/identity.age.key` + `.pub`) and the `~/.dew/images/` directory.

- Refuses to overwrite an existing identity (exit non-zero, key left intact).
- Private key is written `0600`.

```bash
dew keygen
```

### `dew key`
Parent for identity inspection. On its own it lists the `key` subcommands; the one that does the work is `key status`.

### `dew key status`
Report whether an identity is present and show its public key (derives the public key from the private key if the `.pub` file is missing).

```bash
dew key status
```

---

## Repository setup

### `dew init`
Create `.dew/manifest.yaml` in the current directory. Refuses to overwrite an existing manifest.

| Flag | Description |
|---|---|
| `-p, --project <name>` | Project name (default: the directory's base name). Must be path-safe (`[A-Za-z0-9._-]`, ≤64 chars); becomes the image filename `<name>.dew.age`. |
| `--from-gitignore` | Seed the allow-list with discovered candidates (runs `scan` and adds non-noise ignored files). |

Warns if an image with the derived name already exists in `~/.dew/images` (another repo may use the name — consider `--project`).

```bash
dew init
dew init --project billing-svc
dew init --from-gitignore
```

---

## Discovery

### `dew scan`
Read `.gitignore` and walk the working tree to suggest candidate local-only files. Prints **Candidates** (git-ignored, not noise) and **Skipped (noise)**. `.gitignore` is a hint, not an authority — nothing is added automatically.

```bash
dew scan
```

### `dew rules`
Show the effective allow-list and the three deny layers, by source: **built-in**, **global** (`~/.dew/config.yaml`), and **repo** (`.dew/manifest.yaml`).

```bash
dew rules
```

---

## Manifest editing

### `dew add <path>...`
Add one or more paths to the manifest allow-list (deduped). Rejects paths outside the repo and the repo root itself.

`dew add .` is special: it adds **discovered candidates** (from `scan`), prompting `Y/n` per file — not every file in the repo.

| Flag | Description |
|---|---|
| `-y, --yes` | With `add .`, accept all discovered candidates without prompting. |

```bash
dew add .env.local certs/dev.pem
dew add .            # interactive: add discovered candidates
dew add . --yes      # add all discovered candidates
```

### `dew remove <path>...` (alias `rm`)
Remove one or more paths from the allow-list. Removing an untracked path is a clean no-op.

```bash
dew remove .env.local
```

### `dew list` (alias `ls`)
Print the project name and the tracked allow-list.

```bash
dew list
```

---

## Image lifecycle

### `dew pack`
Build the encrypted image: allow-listed files → tar → zstd → age → `~/.dew/images/<project>.dew.age`. Written atomically (temp file + rename). Honors the deny-list (built-in + global + repo) so an allow-listed directory never packs noise.

Requires the manifest, an identity, and that allow-listed paths exist. Refuses to overwrite an image created by a **different** repo (ownership marker mismatch).

| Flag | Description |
|---|---|
| `--dry-run` | List what would be packed (files + sizes + total); write nothing. Needs no identity. |
| `--force` | Overwrite an image created by a different repo. |

```bash
dew pack
dew pack --dry-run
dew pack --force
```

### `dew restore`
Extract the image back into the repo: age decrypt → zstd decompress → tar extract. **Atomic and non-destructive** — staged to a temp dir, then placed; a file that differs from the image is reported as a **conflict and left untouched** (exit non-zero) unless `--force`.

| Flag | Description |
|---|---|
| `--dry-run` | Preview the written / unchanged / conflict / overwrite classification without changing the working tree. |
| `--force` | Overwrite local files that differ from the image. |

```bash
dew restore
dew restore --dry-run
dew restore --force
```

### `dew hydrate`
The same operation as `dew restore` (same flags), surfaced as its own command — dew's signature verb.

```bash
dew hydrate
dew hydrate --dry-run
```

---

## Health & inventory

### `dew status`
Per-repo health: project, identity, manifest validity, image presence, tracked-file count, hydration state, and sync configuration.

```bash
dew status
```

### `dew doctor`
Diagnose the single highest-priority problem and recommend the next action (missing identity/manifest/image, undecryptable image, missing tracked files, …), or report **"Repository fully hydrated."** Verifies the image actually decrypts, not just that it exists.

```bash
dew doctor
```

### `dew images`
Global inventory: list every image in `~/.dew/images` with project, size, last-packed time, and owning repo id. Repo-independent — runs from anywhere.

```bash
dew images
```

---

## Sync

### `dew remote`
Manage the single sync destination (stored in `~/.dew/config.yaml`, shared across all repos). With no subcommand, prints the current destination — or a hint if none is set.

```bash
dew remote                        # show the current destination
dew remote set /Volumes/nas/dew   # set it (local/mounted path)
dew remote set nas:/volume1/dew   # or an scp-style host:path
dew remote unset                  # clear it
```

- `set <dest>` replaces any existing destination; empty/whitespace is rejected.
- `unset` is a no-op if nothing is configured.
- The destination also appears in `dew status`.

### `dew remote test`
Check the configured destination is actually usable before relying on `dew sync`. Exits non-zero if not.

- **Local / mounted:** verifies the directory exists and is writable (or that a missing path is creatable under a writable ancestor) — catching the common "the volume isn't mounted" case.
- **Remote `host:path`:** over `ssh` (`BatchMode`, so it never prompts), reports *reachable*, *trusted* (host key in `known_hosts`), and *path writable* — surfacing OpenSSH's own verdict. An untrusted host key fails with a hint to `ssh <host>` once to accept it.

```bash
dew remote test
```

Requires `ssh` only for remote destinations (graceful "tool not found" otherwise); local checks need nothing.

### `dew remote images`
List the `*.dew.age` images stored at the destination — the mirror of [`dew images`](#dew-images) (which lists this machine's `~/.dew/images`). Confirms a push landed, or shows what a new machine can pull.

- **Local / mounted:** read directly, with size and modified time.
- **Remote `host:path`:** listed over `ssh` (`ls -l`); names and best-effort sizes (the locale-dependent date is shown as `-`).
- A missing or empty destination prints "No images at …".

```bash
dew remote images
```

### `dew sync`
Push the current repo's image to the destination configured with [`dew remote set`](#dew-remote). **Hybrid transport:** local/mounted destinations use a pure-Go copy; remote `host:path` destinations shell out to `scp` (inheriting your `~/.ssh/config`, agent, and `known_hosts`). Sync moves the encrypted image only — never the private key.

```bash
dew sync
```

### `dew sync pull`
Pull the image from the destination into `~/.dew/images/`, then points you at `dew restore`.

```bash
dew sync pull
```

---

## Exit codes

`dew` exits non-zero on error (the message is printed to stderr as `dew: error: …`). Notable cases:

- `dew restore` with unresolved conflicts (no `--force`) — exits non-zero so the conflicts are visible (a `--dry-run` preview always exits 0).
- `dew pack` against an image owned by a different repo (no `--force`).
- `dew keygen` / `dew init` when the target already exists.
- A required external tool is missing — `scp` (`dew sync` to a remote) or `ssh` (`dew remote test`/`images` against a remote); only for remote destinations.
- `dew remote test` when the destination is unusable (unreachable, untrusted host key, or not writable).
