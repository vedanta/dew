# dew

> dew — the local half of your repo, restored after every clone.

**dew** is a local-first CLI that manages the *private* repository state Git intentionally ignores: `.env.local`, dev certificates, `docker-compose.override.yml`, private fixtures, local config — the per-developer files needed to actually run a clone.

Git tracks shared project state. dew tracks the local-only files that make a cloned repo work. It packages an allow-listed set of files into a single encrypted image per repo and can sync that image to a remote, so a fresh clone can be **hydrated** back to a working state.

```bash
git clone <repo> && cd <repo>
dew sync pull   # fetch the encrypted image
dew restore     # extract local files back into the working tree
```

Git gives you the code. dew gives you the missing local context.

## What dew is not

dew is **not** a secrets manager, a backup tool, Git LFS, or a cloud sync service. It is a repo-aware local context manager for files that Git intentionally ignores. Sync copies encrypted images only — never private keys.

## How it works

dew uses a **two-location model**:

- **In the repo (committed to Git):** `.dew/manifest.yaml` declares the project name, image name, and an allow-list (plus an optional `deny:` list). It never contains secrets, file contents, or keys.
- **In your home (never committed):** `~/.dew/` holds `config.yaml`, a single global `age` keypair, and `images/<project>.dew.age` — the encrypted shadow image(s).

There is **one global identity** shared across all repos and **one encrypted image per repo**.

### Architecture

```mermaid
flowchart TB
    subgraph repo["Git repo (committed)"]
        manifest[".dew/manifest.yaml<br/>allow-list + deny-list"]
        working["working tree<br/>.env.local, certs/, overrides…"]
    end

    subgraph home["~/.dew/ (never committed)"]
        config["config.yaml<br/>sync destination"]
        key["identity.age.key (private)"]
        pub["identity.age.pub (public)"]
        image["images/&lt;project&gt;.dew.age<br/>encrypted shadow"]
    end

    remote[("Sync destination<br/>nas:/volume1/dew")]

    manifest -->|"selects files"| working
    working -->|"dew pack"| image
    pub -->|"encrypts"| image
    image -->|"dew restore"| working
    key -->|"decrypts"| image
    image <-->|"dew sync / sync pull"| remote
    config -.->|"configures"| remote
```

### Packaging pipeline

```
Pack:    allow-listed files → tar → zstd → age encrypt → ~/.dew/images/<project>.dew.age
Restore: image → age decrypt → zstd decompress → tar extract → write into repo
```

The allow-list is authoritative — `pack` only ever includes paths the manifest lists, never "everything ignored." A deny-list (built-in patterns + per-manifest `deny:`) keeps noise like `node_modules/`, `dist/`, and `*.log` out. `.gitignore` is only a hint for discovery.

### Workflow

```mermaid
sequenceDiagram
    actor Dev as Developer
    participant Repo as Git repo
    participant CLI as dew
    participant Store as ~/.dew/
    participant Remote as Sync destination

    Note over Dev,Remote: First-time setup (author machine)
    Dev->>CLI: dew keygen
    CLI->>Store: create age identity
    Dev->>CLI: dew init / scan / add
    CLI->>Repo: write .dew/manifest.yaml
    Dev->>CLI: dew pack
    CLI->>Store: tar → zstd → age → image
    Dev->>CLI: dew sync
    CLI->>Remote: push encrypted image
    Dev->>Repo: git commit manifest & push

    Note over Dev,Remote: Hydrate a fresh clone (new machine)
    Dev->>Repo: git clone && cd
    Dev->>CLI: dew sync pull
    Remote->>Store: fetch encrypted image
    Dev->>CLI: dew restore
    Store->>Repo: decrypt → decompress → extract
    Dev->>CLI: dew doctor
    CLI-->>Dev: Repository fully hydrated.
```

## Command set (MVP)

```bash
# Identity
dew keygen                      # create the global age identity
dew key status                  # inspect identity

# Repository setup
dew init [--from-gitignore]     # create .dew/manifest.yaml

# Discovery
dew scan                        # suggest candidate local files

# Manifest
dew add <path> | add .          # add file/dir/discovered candidates
dew remove <path> | list        # edit / view the allow-list

# Image lifecycle
dew pack | restore              # build / extract the encrypted image

# Health
dew status | doctor             # validate hydration state

# Sync
dew sync | sync pull            # push / pull the encrypted image
```

## Example end-to-end flow

```bash
# One-time setup
dew keygen

# In a repo
cd myrepo
dew init
dew scan
dew add .env.local
dew add docker-compose.override.yml
dew pack
git add .dew/manifest.yaml && git commit -m "Add dew manifest" && git push
dew sync

# On a new machine
git clone <repo> && cd myrepo
dew sync pull
dew restore
dew doctor   # → Repository fully hydrated.
```

## Status

This repository currently contains the design spec only — implementation has not started. The authoritative MVP spec is [`docs/design.md`](docs/design.md).

## Tech

Go single binary · [Cobra](https://github.com/spf13/cobra) · `gopkg.in/yaml.v3` · `archive/tar` · zstd · [age](https://github.com/FiloSottile/age) · scp/rsync.
