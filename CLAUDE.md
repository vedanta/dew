# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

**This repository contains design specs only — no code exists yet.** The implementation has not started. The authoritative MVP spec is `docs/design.md` (consolidated and renamed from the original `idea/design-1.md` + `idea/design-2.md`, which are now git-ignored scratch notes). The product was renamed from **ungit** to **dew**; see `docs/design.md` §26 for the rationale. When you begin implementing, you are creating the project structure from scratch per `docs/design.md`.

## What dew is

dew is a local-first CLI that manages the *private* repository state Git intentionally ignores (`.env.local`, dev certs, `docker-compose.override.yml`, private fixtures, local config). Git tracks shared project state; dew tracks the per-developer local files needed to actually run a clone. It packages an allow-listed set of files into a single encrypted image per repo, and can sync that image to a remote destination so a fresh clone can be "hydrated" back to a working state.

The defining workflow:
```bash
git clone <repo> && cd <repo>
dew sync pull   # fetch the encrypted image
dew restore     # extract local files back into the working tree
```

dew is explicitly **not** a secrets manager, backup tool, Git LFS, or cloud sync service. Keep the MVP narrow (see "Scope discipline").

## Intended architecture

Language: **Go**, distributed as a single cross-platform binary. Stack: Cobra (CLI), `gopkg.in/yaml.v3` (config + manifest), `archive/tar`, zstd compression, `age` for encryption (shell out to the `age` CLI first; consider the native library later), and scp/rsync shell-out for sync.

Planned package layout:
```
cmd/        one file per command (root, init, scan, add, remove, list, pack, restore, status, doctor, sync, key)
internal/
  config/    read/write ~/.dew/config.yaml
  manifest/  read/write <repo>/.dew/manifest.yaml
  identity/  create/inspect the global age identity
  scanner/   read .gitignore + walk working dir to discover candidate files
  archive/   build and extract tar archives
  crypto/    age encrypt/decrypt
  restore/   safely restore files into the repo
  sync/      push/pull encrypted images
  doctor/    validate hydration state and recommend next actions
```

### Two-location model (central to the design)

- **In the repo (committed to Git):** `.dew/manifest.yaml` — declares project name, image name, and the allow-list (and an optional `deny:` list). It never contains secrets, file contents, or keys.
- **In the user's home (never committed):** `~/.dew/` holds `config.yaml`, the `identity.age.key` / `identity.age.pub` keypair, and `images/<project>.dew.age` — the encrypted shadow image(s).

There is **one global identity** (one keypair) shared across all repos, and **one encrypted image per repo**. No per-repo keys, no multiple recipients in the MVP.

### Packaging pipeline

Pack: allow-listed files → tar → zstd → age encrypt → `~/.dew/images/<project>.dew.age`.
Restore: image → age decrypt → zstd decompress → tar extract → write into repo (preserve permissions, warn before overwriting).

### Allow-list authoritative, deny-list for safety, `.gitignore` only a hint

`pack` only ever includes paths in the manifest allow-list — never "everything ignored." A deny-list (built-in patterns + per-manifest `deny:`) guarantees noise stays out even when a directory is added. `scan` reads `.gitignore` and the working tree to *suggest* candidates, but the user explicitly opts each in. `dew add .` means "add discovered candidates," **not** "add every file in the repo." Always exclude noise like `node_modules/`, `dist/`, `build/`, `target/`, `.venv/`, `__pycache__/`, `.DS_Store`, `*.log`.

## Command set (MVP)

```
dew keygen | key status                 # identity
dew init [--from-gitignore]             # create .dew/manifest.yaml
dew scan                                # discover candidate files
dew add <path> | add . | remove <path> | list   # manifest editing
dew pack | restore                      # image lifecycle
dew status | doctor                     # health checks
dew sync | sync pull                    # push/pull encrypted image
```

`docs/design.md` §23 prescribes the build order: (1) manifest commands, (2) identity, (3) pack/restore, (4) discovery, (5) health, (6) sync. Follow this sequence — pack/restore is the core value and should work end-to-end before adding discovery polish.

## Scope discipline

The MVP is deliberately minimal. Do **not** add (these are explicitly deferred): version history, snapshot messages, team sharing, multiple recipients, per-repo keys, key rotation/export, cloud provider integration, GitHub integration, image diffing, conflict resolution, GUI, or any server/account system. Sync copies encrypted images only — never private keys.

## Implementation notes worth getting right early

These survive the local-only MVP and are painful to retrofit (see conversation history that motivated them):

- **Path sanitization on restore.** Reject `..` and symlink entries during tar extraction so an image can never write outside the repo (tar-slip / symlink escape).
- **Atomic, non-destructive restore.** Extract to a temp dir then move into place; compare (hash/mtime) before overwriting rather than just warning — "no version history" means a careless restore is silent data loss.

## Open questions (left open in docs/design.md §27, resolve during refinement)

- **Image filename extension:** examples use `<project>.dew.age`, but `<project>.dew` (no suffix) is the alternative from one source spec. Not locked.
- **Deny-list scope:** whether the *built-in* deny layer ships in v1 or only the per-manifest `deny:` block.
