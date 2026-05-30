# dew — MVP Design Specification

> **Status:** MVP design, consolidated from the original `idea/design-1.md` and
> `idea/design-2.md`. This document preserves the original MVP design unchanged —
> it is renamed (`ungit` → `dew`) and merged, not redesigned. Refinements will
> happen later; open points where the two source docs differed are listed in the
> [Open Questions](#26-open-questions-to-refine-later) appendix rather than being
> silently resolved.

## 1. Product Summary

**dew** is a local-first developer tool that manages the private repository state that Git intentionally ignores.

Git tracks the source code, documentation, and shared project history. dew tracks the local-only files needed to make a cloned repository actually work, such as `.env.local`, certificates, local Docker overrides, private fixtures, and developer-specific configuration.

The core goal of dew is simple:

> Clone a repo, pull the private repo shadow, restore it, and get a working development environment.

## 2. Core Product Definition

**Git manages shared repository state.**

**dew manages private repository state.**

dew does not replace Git. It complements Git by managing the files that should not be committed but are still essential to local development.

Example:

```bash
git clone <repo>
cd <repo>

dew sync pull
dew restore
```

After restore, the repo has its local development context back.

## 3. Problem Statement

A Git repo often does not contain everything required to run the project locally.

Git usually includes:

```text
source code
README files
package manifests
infrastructure templates
tests
documentation
```

But Git intentionally excludes:

```text
.env.local
private certificates
local config files
docker-compose.override.yml
private fixtures
local database seed data
developer-specific IDE settings
```

These files are often excluded through `.gitignore`, which prevents accidental publishing but creates a different problem: they are hard to recover, move across machines, or restore after cloning a repo.

Today, developers solve this manually with:

```text
Slack messages
1Password notes
copy-pasted .env files
private folders
manual setup docs
Dropbox folders
ad hoc shell scripts
```

dew turns this hidden local state into a first-class, encrypted, repo-aware artifact.

## 4. MVP Design Philosophy

The MVP should remain intentionally small.

dew v1 is not a cloud platform, team sharing tool, secret manager, or Git replacement. It is a local developer utility.

MVP principles:

```text
Local-first
Solo-user only
One global identity
One encrypted image per repo
Explicit allow-list
Built-in deny-list
No version history
No team sharing
No cloud account
No GUI
Single binary distribution
```

The MVP should answer one question well:

> What local files does this repo need, and can I restore them safely?

## 5. Conceptual Model

dew introduces the idea of a **repo shadow**.

```text
Git repo     = shared project state
dew shadow   = private local project state
```

The repo shadow is an encrypted archive containing the local-only files listed in the dew manifest.

## 6. Repository Layout

Inside the Git repo:

```text
repo/
├── .git/
├── .gitignore
├── .dew/
│   └── manifest.yaml
├── src/
└── README.md
```

The `.dew/manifest.yaml` file is committed to Git.

It describes which local files dew should manage, but it does not contain the actual private files or secrets.

## 7. Local dew Layout

dew stores private data under the user's home directory:

```text
~/.dew/
├── config.yaml
├── identity.age.key
├── identity.age.pub
└── images/
    ├── liway.dew.age
    ├── foai.dew.age
    └── jot.dew.age
```

### Local Files

```text
~/.dew/identity.age.key
```

Private key. Used to decrypt images. Must never be committed.

```text
~/.dew/identity.age.pub
```

Public key. Used to encrypt images.

```text
~/.dew/images/<project>.dew.age
```

Encrypted repo shadow image.

```text
~/.dew/config.yaml
```

Global dew configuration, including default sync destination.

## 8. Manifest Design

The manifest is the repo-level contract.

Example:

```yaml
version: 1
project: liway
image: liway.dew.age

allow:
  - .env.local
  - docker-compose.override.yml
  - certs/dev.pem
  - fixtures/private-data.json

deny:
  - node_modules/
  - dist/
  - build/
  - .next/
  - target/
  - .venv/
  - __pycache__/
  - "*.log"
```

### Manifest Responsibilities

The manifest defines:

```text
project name
image name
allowed files and folders
denied files and folders
restore targets
```

The manifest does not store:

```text
secrets
private file contents
private keys
encrypted image contents
```

## 9. Allow-List Model

dew uses an explicit allow-list model.

This is a key MVP decision.

dew should not blindly package everything in `.gitignore`.

Instead:

```bash
dew add .env.local
dew add certs/dev.pem
dew add docker-compose.override.yml
```

These commands add entries to the manifest.

Then:

```bash
dew pack
```

only packages files included in the manifest allow-list.

This avoids accidentally capturing:

```text
node_modules/
dist/
build/
logs/
production credentials
temporary files
large cache directories
```

## 10. Deny-List Model

In addition to the explicit allow-list, dew maintains a deny-list of files and folders that are **never** considered during discovery or packaging, even if a glob or directory in the allow-list would otherwise sweep them in.

The deny-list has two layers:

- **Built-in deny patterns**, shipped with dew, covering the usual heavy or generated paths:

  ```text
  node_modules/
  dist/
  build/
  target/
  .venv/
  __pycache__/
  .DS_Store
  ```

- **Per-manifest deny entries**, declared under `deny:` in `.dew/manifest.yaml`, for project-specific exclusions (for example `*.log` or `.next/`).

The deny-list is what makes a future "add a whole directory" convenient and safe: the allow-list says what to include, and the deny-list guarantees the noise stays out.

## 11. Gitignore Integration

dew can use `.gitignore` as a discovery source.

Command:

```bash
dew scan
```

or:

```bash
dew init --from-gitignore
```

dew reads `.gitignore`, checks the working directory, and identifies likely candidates.

Example output:

```text
Discovered ignored files:

Suggested candidates:
  .env.local
  .env.dev
  certs/dev.pem
  docker-compose.override.yml

Skipped:
  node_modules/
  dist/
  build/
  .DS_Store
  __pycache__/
```

The user then chooses what to add.

Important rule:

> `.gitignore` is a hint, not an authority.

## 12. Encryption Model

MVP uses one global solo identity.

```text
One user
One keypair
Many repos
```

The identity is stored locally:

```text
~/.dew/identity.age.key
~/.dew/identity.age.pub
```

### Recommended Encryption Tooling

Use **age** as the encryption mechanism.

dew can either shell out to the `age` CLI initially or later use a native Go age library.

### Why age Instead of PGP/GPG

dew's mental model is similar to PGP:

```text
public key encrypts
private key decrypts
```

But PGP/GPG introduces too much ceremony for the MVP.

Age is preferred because it is:

```text
modern
simple
file-encryption focused
easier to automate
better suited for developer tooling
```

## 13. Packaging Format

dew image format:

```text
tar + zstd + age
```

Logical process:

```text
selected files
→ tar archive
→ zstd compression
→ age encryption
→ ~/.dew/images/<project>.dew.age
```

Restore process:

```text
encrypted image
→ age decrypt
→ zstd decompress
→ tar extract
→ restore files into repo
```

## 14. Sync Model

MVP sync is simple and scp-style.

dew sync does not manage Git remotes and does not push to GitHub.

It copies the encrypted image to a configured destination.

Global config:

```yaml
sync:
  destination: nas:/volume1/dew
```

Push current project image:

```bash
dew sync
```

Pull current project image:

```bash
dew sync pull
```

The default sync operation should sync only encrypted images, not private keys.

Private key backup is intentionally out of scope for MVP.

## 15. MVP Command Set

### 15.1 Identity Commands

#### Create global identity

```bash
dew keygen
```

Creates:

```text
~/.dew/identity.age.key
~/.dew/identity.age.pub
```

Rules:

```text
refuse to overwrite existing identity
create ~/.dew if missing
create ~/.dew/images if missing
```

#### Check identity

```bash
dew key status
```

Example output:

```text
Identity: Present
Private Key: ~/.dew/identity.age.key
Public Key: age1...
```

### 15.2 Repository Setup Commands

#### Initialize dew in repo

```bash
dew init
```

Creates:

```text
.dew/manifest.yaml
```

#### Initialize and scan from gitignore

```bash
dew init --from-gitignore
```

Creates manifest and runs candidate discovery.

### 15.3 Discovery Commands

#### Scan repo

```bash
dew scan
```

Scans `.gitignore` and the working directory for likely local-only files.

Output:

```text
Candidates:
  .env.local
  certs/dev.pem
  docker-compose.override.yml

Skipped:
  node_modules/
  dist/
  build/
```

### 15.4 Manifest Commands

#### Add a file

```bash
dew add .env.local
```

Adds the file to the manifest allow-list.

#### Add a directory

```bash
dew add certs/
```

Adds the directory to the manifest allow-list.

#### Add all discovered candidates

```bash
dew add .
```

Important: this should not mean "add everything in the repo."

It should mean:

> Add all discovered dew candidates.

In interactive mode, it can ask:

```text
Add .env.local? [Y/n]
Add certs/dev.pem? [Y/n]
Add docker-compose.override.yml? [Y/n]
```

#### Remove a file

```bash
dew remove .env.local
```

Removes a path from the manifest allow-list.

#### List managed files

```bash
dew list
```

Example output:

```text
Project: liway

Tracked:
  .env.local
  docker-compose.override.yml
  certs/dev.pem
```

### 15.5 Image Commands

#### Pack image

```bash
dew pack
```

Creates:

```text
~/.dew/images/<project>.dew.age
```

The command should:

```text
read .dew/manifest.yaml
validate identity exists
validate allowed files exist
archive allowed files
compress archive
encrypt archive
write image to ~/.dew/images/
```

#### Restore image

```bash
dew restore
```

Restores local files into the repo.

The command should:

```text
read .dew/manifest.yaml
find image in ~/.dew/images/
decrypt image
extract files into repo
preserve file permissions when possible
warn before overwriting existing files
```

### 15.6 Health Commands

#### Status

```bash
dew status
```

Example output:

```text
Project: liway

Identity: Present
Manifest: Valid
Image: Present
Tracked Files: 3
Hydration: Healthy
Sync: Not configured
```

#### Doctor

```bash
dew doctor
```

Diagnoses problems and recommends next action.

Example output:

```text
Problem:
  .env.local is missing

Image:
  Present

Recommended action:
  Run 'dew restore'
```

Potential checks:

```text
missing identity
missing manifest
invalid manifest
missing image
missing tracked files
image exists but cannot decrypt
sync destination missing
tracked file listed but not found during pack
```

### 15.7 Sync Commands

#### Push current repo image

```bash
dew sync
```

Copies current project image to the configured sync destination.

#### Pull current repo image

```bash
dew sync pull
```

Copies current project image from the configured sync destination into:

```text
~/.dew/images/
```

## 16. Final MVP Command List

```bash
# Identity
dew keygen
dew key status

# Repository setup
dew init
dew init --from-gitignore

# Discovery
dew scan

# Manifest
dew add <path>
dew add .
dew remove <path>
dew list

# Image lifecycle
dew pack
dew restore

# Health
dew status
dew doctor

# Sync
dew sync
dew sync pull
```

## 17. Commands Explicitly Deferred

The following are not part of the MVP:

```bash
dew pack -m "message"
dew history
dew restore --version
dew key rotate
dew key export
dew team add
dew share
dew diff
dew merge
dew cloud
```

## 18. Out of Scope for MVP

The following capabilities are intentionally deferred:

```text
version history
snapshot messages
team sharing
multiple recipients
per-repo keys
key rotation
cloud provider integrations
GitHub integration
secret rotation
conflict resolution
image diffing
GUI
web service
account system
central server
```

## 19. Language Recommendation

dew should be implemented in **Go** for the MVP.

Python would be faster for a personal proof-of-concept, but Go better fits the long-term product shape.

## 20. Why Go

dew is a CLI-first developer tool.

Its core responsibilities are:

```text
filesystem walking
manifest parsing
archive creation
compression
encryption orchestration
restore operations
sync operations
single binary distribution
cross-platform execution
```

These are natural strengths for Go.

### 20.1 Single Binary Distribution

A major advantage of Go is that dew can be shipped as a single binary:

```bash
curl -L <release-url>/dew-darwin-arm64 -o dew
chmod +x dew
```

No Python installation issues.

No virtual environments.

No pip dependency conflicts.

No package manager assumptions.

This matters because dew's value proposition is:

> Drop this tool anywhere and hydrate a repo.

### 20.2 Cross-Platform Support

Go can easily produce binaries for:

```text
macOS arm64
macOS amd64
Linux amd64
Linux arm64
Windows amd64
```

This is important for a developer tool that may eventually run on many machines.

### 20.3 Strong Standard Library

Go has good built-in support for:

```text
filesystem operations
path handling
tar archives
process execution
streams
error handling
```

With a few focused dependencies, it can handle the full MVP cleanly.

### 20.4 Better Operational Feel

For a tool that handles secrets and local files, a compiled binary feels more predictable than a Python script with many runtime dependencies.

This does not automatically make it safer, but it makes distribution, installation, and reproducibility cleaner.

## 21. Recommended Go Stack

Suggested stack:

```text
Language: Go
CLI framework: Cobra
Config: simple YAML first, Viper later if needed
Manifest parser: gopkg.in/yaml.v3
Archive: archive/tar
Compression: zstd
Encryption: age CLI first or native age library
Sync: scp/rsync shell-out for MVP
```

MVP should avoid overengineering.

A practical first version can shell out to:

```text
age
scp
```

Then later replace with native libraries if needed.

## 22. Proposed Internal Architecture

Suggested package structure:

```text
dew/
├── cmd/
│   ├── root.go
│   ├── init.go
│   ├── scan.go
│   ├── add.go
│   ├── remove.go
│   ├── list.go
│   ├── pack.go
│   ├── restore.go
│   ├── status.go
│   ├── doctor.go
│   ├── sync.go
│   └── key.go
├── internal/
│   ├── config/
│   ├── manifest/
│   ├── identity/
│   ├── scanner/
│   ├── archive/
│   ├── crypto/
│   ├── restore/
│   ├── sync/
│   └── doctor/
├── go.mod
└── README.md
```

### Package Responsibilities

```text
config
  Read/write ~/.dew/config.yaml

manifest
  Read/write .dew/manifest.yaml

identity
  Create and inspect global age identity

scanner
  Read .gitignore and discover candidate files

archive
  Build tar archive and extract archive

crypto
  Encrypt/decrypt using age

restore
  Restore files safely into repo

sync
  Push/pull encrypted images

doctor
  Validate repo hydration and recommend actions
```

## 23. MVP Build Sequence

A disciplined implementation order:

### Phase 1: Repo and Manifest

```text
dew init
dew add <path>
dew remove <path>
dew list
```

### Phase 2: Identity

```text
dew keygen
dew key status
```

### Phase 3: Pack and Restore

```text
dew pack
dew restore
```

### Phase 4: Discovery

```text
dew scan
dew init --from-gitignore
dew add .
```

### Phase 5: Health

```text
dew status
dew doctor
```

### Phase 6: Sync

```text
dew sync
dew sync pull
```

## 24. Example End-to-End Flow

First-time setup:

```bash
dew keygen
```

Inside repo:

```bash
cd liway

dew init
dew scan
dew add .env.local
dew add docker-compose.override.yml
dew add certs/dev.pem
dew pack
```

Commit manifest:

```bash
git add .dew/manifest.yaml
git commit -m "Add dew manifest"
git push
```

Sync private shadow:

```bash
dew sync
```

On a new machine:

```bash
git clone <repo>
cd liway

dew key status
dew sync pull
dew restore
dew doctor
```

Expected result:

```text
Repository fully hydrated.
```

## 25. README-Level Pitch

dew manages the files your repo needs but Git should never commit.

```bash
git clone <repo>
cd <repo>

dew sync pull
dew restore
```

Git gives you the code.

dew gives you the missing local context.

### Final Positioning

dew is not a secrets manager.

dew is not a backup tool.

dew is not Git LFS.

dew is not a cloud sync service.

dew is:

> A repo-aware local context manager for files that Git intentionally ignores.

The MVP is strong because it solves a narrow, painful developer problem with boring, reliable primitives:

```text
Go
YAML
tar
zstd
age
scp
```

That is enough to build something useful without turning it into a platform too early.

## 26. Why the name "dew"

The product was originally called **ungit**. The name was changed to **dew** for both meaning and practicality.

### The metaphor fits the product

The success state of the tool, stated throughout this spec, is:

> Repository fully hydrated.

**Dew** is the thin layer of water that settles back onto a surface — it reappears, quietly, and brings something dry back to life. That is exactly what the tool does: after a fresh `git clone`, the repo is dry — it has the code but none of the local context. `dew restore` settles the missing local files back onto it. The verb *hydrate* and the noun *dew* describe the same act, so the name and the product reinforce each other instead of needing explanation.

`dew restore` reads as plain English for what it does.

### It avoids the crowded, misleading `*git` namespace

The candidates anchored to Git — `ungit`, `ugit`, `sgit` — all failed on two counts:

- **Collision.** Each is already taken, often several times over. `ungit` is a well-known web-based Git GUI; `ugit` is a popular "undo git" tool with the `brew install ugit` slot already claimed; `sgit` exists as multiple git helpers. The install and discovery namespaces were not available.
- **Wrong signal.** Anything in the `*git` family reads as "a Git wrapper" or even "anti-Git" / "undo Git" — which actively fights this product's core positioning: *dew complements Git, it does not replace it.* A neutral name lets the tool stand on its own.

### It is practical to ship

- **Short and typeable.** Three letters, fast at the command line: `dew pack`, `dew restore`.
- **Pronounceable and memorable.** It passes the say-it-out-loud test (unlike disemvoweled or two-letter options), so it survives demos, docs, and word-of-mouth.
- **Namespace is clear where it counts.** For a single Go binary the relevant channels are Homebrew, `go install`, and direct download — `brew install dew` is unclaimed and no dominant developer CLI owns the name. (npm is irrelevant for a Go binary, which removed the constraint that sank water-themed shorthands like `wtr`.)
- **No clash with a standard command.** There is no common Unix command named `dew`.

The one known cost is general-web SEO: "dew" is a common English word, so early search results will be noisy. This is mitigated with a clear tagline and an unambiguous repo/domain slug, and it is a far smaller problem than launching into an already-occupied tool namespace.

### Naming conventions that follow

```text
binary:          dew
repo manifest:   .dew/manifest.yaml
local store:     ~/.dew/
image file:      ~/.dew/images/<project>.dew.age
tagline:         "dew — the local half of your repo, restored after every clone."
```

## 27. Open Questions (to refine later)

These points differed between the two original design docs and are intentionally left open — the MVP design is not being changed here, only consolidated. Resolve them during refinement:

- **Image filename extension.** The detailed source spec used `<project>.dew.age` (encryption suffix explicit); the second source spec used `<project>.dew` (no suffix). This document uses `<project>.dew.age` consistently in examples, but the convention is not yet locked.
- **Deny-list scope.** The deny-list (Section 10) came from the second source spec; the first did not include one. It is documented here as part of the MVP because it makes directory-level adds safe, but whether the *built-in* layer ships in v1 or only the per-manifest `deny:` block is still open.
