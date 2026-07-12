# FAQ

## What is dew?

**dew** is a local-first CLI for the private, local files that make a cloned repository actually work — the files Git usually ignores on purpose.

Examples include:

- `.env.local`
- development certificates
- `docker-compose.override.yml`
- private fixtures
- local notes
- machine-specific config
- sandbox credentials
- local-only scripts or overrides

Git gives you the shared code. **dew gives you the missing local context.**

## What problem does dew solve?

A Git repo often does not contain everything needed to run the project locally.

That is usually intentional. You do not want to commit secrets, private config, certificates, or machine-specific overrides. But the result is familiar:

```bash
git clone <repo>
cd <repo>
npm install
docker compose up
# ...and then something breaks because local files are missing
```

dew solves the "fresh clone is not really runnable" problem by letting you package selected local-only files into an encrypted image and restore them later.

## Is dew a secrets manager?

No.

dew is **not** a secrets manager. It does not provide secret leasing, secret rotation, access policies, audit trails, approval workflows, cloud IAM integration, or per-environment secret distribution.

dew is closer to a **repo-aware local context manager**.

It helps you preserve and restore files that already exist on your machine. Some of those files may contain secrets, but dew itself is not trying to replace tools like:

- 1Password
- Bitwarden
- Vault
- AWS Secrets Manager
- Doppler
- SOPS
- Chamber
- direnv
- Kubernetes Secrets
- cloud-native secret stores

Use a real secrets manager when you need team-wide policy, rotation, centralized access control, audit, and production-grade governance.

Use dew when you need your private local repo context to survive across clones and machines.

## Is dew a backup tool?

No.

dew is not a general backup tool. By default it only manages files that you explicitly allow-list in a repo manifest.

It does not back up your whole home directory, your Git history, your IDE, your database, or arbitrary machine state. (`dew pack --all` can sweep one repo's *working copy* into an image as a one-shot — for relocating to another machine, not as a backup strategy.)

dew creates one encrypted image per repo. That image can be synced somewhere else, but dew does not replace a real backup strategy.

Think of dew as:

> "The small missing local layer for this repo."

Not:

> "A complete backup system for my computer."

## Is dew a cloud sync service?

No.

dew can copy encrypted images to a configured destination, but dew is not a cloud storage provider.

It does not host your files. It does not run a server. It does not give you an account. It does not manage a hosted backend.

You choose where encrypted images go. That might be:

- a NAS
- another machine over SSH
- a mounted drive
- a shared local path
- another storage location supported by your workflow

dew moves encrypted images. It does **not** move your private key during normal sync.

## How does dew work?

dew uses a two-location model.

In your repo, dew stores a committed manifest:

```text
.dew/manifest.yaml
```

That manifest says which local files dew should manage. It contains metadata only. It should not contain secrets, file contents, or private keys.

In your home directory, dew stores private local state:

```text
~/.dew/
```

That includes:

```text
~/.dew/config.yaml
~/.dew/identity.age.key
~/.dew/identity.age.pub
~/.dew/images/<project>.dew.age
```

The basic flow is:

```bash
dew keygen
dew init
dew add .env.local
dew pack
dew remote set <destination>
dew sync
```

On another machine:

```bash
dew key pull <user@host>
dew remote set <destination>
dew sync pull
dew restore
```

## What is the encrypted image?

The encrypted image is the packed form of your allow-listed local files.

The pipeline is:

```text
allow-listed files -> tar -> zstd compression -> age encryption -> .dew.age image
```

The result is stored locally under:

```text
~/.dew/images/<project>.dew.age
```

This is the file that `dew sync` copies.

## What is the manifest?

The manifest is the repo-level contract for dew.

It lives at:

```text
.dew/manifest.yaml
```

It usually contains:

```yaml
version: 1
project: myrepo
image: myrepo.dew.age
id: 1234567890abcdef1234567890abcdef
allow:
  - .env.local
  - docker-compose.override.yml
  - certs/
deny:
  - "*.log"
  - tmp/
```

The manifest is safe to commit because it contains file names and rules, not the file contents.

## Why is the manifest committed to Git?

Because the manifest describes the repo's expected local context.

A new clone needs to know:

- what files are managed by dew
- what image name to look for
- what should be restored
- what should be ignored even inside allow-listed directories

The manifest is shared project metadata. The actual private files are not.

## Does dew commit my secrets?

No.

dew does not commit your secrets to Git.

The only repo file dew commits is the manifest, which describes paths and rules. The file contents are packed into an encrypted image stored outside the repo under `~/.dew/images`.

That said, you still need to be careful. If you manually add secret files to Git, dew cannot protect you from that. Keep your `.gitignore` sane and review commits before pushing.

## Does dew use `.gitignore`?

`.gitignore` is useful input, but it is not the source of truth.

dew's source of truth is the manifest allow-list.

That means dew does not blindly pack "everything ignored by Git." That would be dangerous and noisy.

Instead, you explicitly choose what dew manages:

```bash
dew add .env.local
dew add docker-compose.override.yml
dew add certs/
```

The allow-list is authoritative. (The one deliberate exception is `dew pack
--all`, which packs the whole repo for that single run — the deny list still
applies, and the manifest is untouched.)

## What is the deny list?

The deny list prevents noisy or unsafe files from being packed, especially when you allow-list a directory.

For example, if you allow-list:

```bash
dew add local/
```

you may still want to exclude:

```yaml
deny:
  - "*.log"
  - tmp/
  - node_modules/
```

dew uses multiple deny layers:

1. built-in deny patterns
2. global deny rules in `~/.dew/config.yaml`
3. repo deny rules in `.dew/manifest.yaml`

You can inspect the effective rules with:

```bash
dew rules
```

## Why not just commit `.env.example`?

You should commit `.env.example`.

But `.env.example` does not solve the whole problem.

An example file documents expected variables. It does not preserve your actual local values, dev credentials, local certificates, private fixtures, or machine-specific overrides.

A good workflow is:

```text
.env.example     -> committed documentation
.env.local       -> private local file managed by dew
```

Use both.

## Why not just put secrets in Git encrypted with git-crypt?

`git-crypt` is a good tool for a different workflow.

With `git-crypt`, encrypted secret files live inside the Git repo. That can be useful when the team intentionally wants encrypted files versioned in Git.

dew takes a different stance:

- Git tracks shared code and shared metadata.
- dew manages private local files outside Git.
- The encrypted image lives under `~/.dew/images`.
- The repo only contains the manifest.

Use `git-crypt` when you want encrypted files as part of the repository history.

Use dew when you want the repo to stay clean while still being able to restore local-only context after a clone.

## Why not use SOPS?

SOPS is excellent for encrypting structured secret files, especially YAML, JSON, ENV, and Kubernetes-related workflows.

dew is not trying to replace SOPS.

SOPS is good when you want encrypted secrets to be versioned, reviewed, and deployed as part of infrastructure workflows.

dew is good when you have local-only repo files that should not live in Git at all, even encrypted.

Examples where dew fits better:

- dev-only certificates
- local Docker overrides
- private fixture files
- personal `.env.local`
- local notes
- machine-specific app config
- files that are not clean structured secret documents

## Why not use 1Password or Bitwarden?

You should use a password manager for credentials.

But password managers usually do not restore a full repo-local working state.

They can store the value of `DATABASE_URL`, but they do not naturally restore:

```text
.env.local
docker-compose.override.yml
certs/dev.pem
certs/dev-key.pem
fixtures/private-customer-sample.json
.local/notes.md
```

dew is file-oriented. Password managers are secret-record-oriented.

They can work together: keep your most important credentials in a password manager, and use dew for local repo context files that need to exist on disk.

## Why not use direnv?

`direnv` is great for loading environment variables when you enter a directory.

dew solves a different problem.

`direnv` helps activate local environment state.

dew helps preserve and restore local files.

They can work together:

```text
.envrc        -> committed or local direnv config
.env.local    -> private values managed by dew
```

## Why not use Docker secrets or Kubernetes secrets?

Those tools are for runtime environments.

dew is for local developer repo context.

Docker secrets and Kubernetes secrets help applications access secrets while running. dew helps a developer restore the local files needed before they can even run the app.

## Is dew for teams or solo developers?

Both, but the first version is especially useful for solo developers and small teams.

Solo developers use dew to move between laptops, desktops, dev boxes, and fresh clones.

Small teams can commit a shared `.dew/manifest.yaml` so everyone agrees on which local files are needed, while each developer keeps their own encrypted image and identity.

For larger teams, dew should be used carefully. It does not replace enterprise secrets management, access control, or audit tooling.

## Can multiple developers use the same manifest?

Yes.

The manifest can be committed to Git and shared by the team.

Each developer can have their own private local files and their own encrypted image. The manifest tells dew what types of files matter for the repo. The actual file contents remain private.

## Does every developer share the same encrypted image?

Not necessarily.

The simplest model is one encrypted image per repo per identity.

For personal use, that is straightforward.

For a team, each developer may maintain their own image because their local values may differ. dew is not a multi-user secret distribution system, and intentionally so.

## Where is my private key stored?

dew stores the private age identity locally under:

```text
~/.dew/identity.age.key
```

The public key is stored under:

```text
~/.dew/identity.age.pub
```

The private key is what decrypts your images. Protect it like any other private key.

## Does `dew sync` copy my private key?

No.

`dew sync` moves encrypted images only.

It does not copy:

```text
~/.dew/identity.age.key
```

That is intentional. If a remote destination is compromised, the attacker should only get encrypted images, not the key needed to decrypt them.

To move your identity to another machine, use explicit key transfer commands such as:

```bash
dew key push <user@host>
dew key pull <user@host>
```

Those commands are intentionally separate from sync.

## What happens if I lose my key?

If you lose the private key, you cannot decrypt images encrypted to that identity.

dew cannot recover encrypted images without the key.

That is the point of encryption.

You should keep a safe backup of your identity if the data matters. dew deliberately stays out of key management, so treat:

```text
~/.dew/identity.age.key
```

as sensitive material and back it up carefully yourself — a password manager, encrypted drive, or other secure storage.

## Can I rotate the key?

There is no built-in rotation command — key rotation is intentionally out of dew's scope.

dew uses one global identity for images. If you need to rotate manually, the safe path is:

1. restore files with the old key
2. generate or install a new identity
3. repack images with the new identity
4. resync images
5. update other machines carefully

If you need managed rotation, lease, and revocation, that is what a real secrets manager is for.

## Is the encrypted image safe to upload to cloud storage?

The image is encrypted, so it is designed to be safe to store outside your machine.

However, "encrypted" does not mean "careless."

A good security posture is:

- keep the private key off the remote
- use trusted storage
- restrict access where possible
- avoid syncing files you do not need
- rotate credentials if you suspect exposure
- do not treat dew as a compliance system

dew protects image contents with encryption. It does not provide cloud access control, audit, or key management.

## Can I move an image by hand instead of using `dew sync`?

Yes. The image is one encrypted file — copy it however you like (scp, USB
stick, a shared drive), then restore straight from it on the other machine:

```bash
# machine A
dew pack
scp ~/.dew/images/my-app.dew.age you@machineB:~/

# machine B (in the cloned repo; your identity already there)
dew restore --image ~/my-app.dew.age
```

`--image` needs no manifest and no sync destination; the usual non-destructive
restore rules apply. If the machine will keep using dew for that repo, move the
file into `~/.dew/images/` instead so plain `restore`/`pack`/`sync` work from
then on.

## Can I inspect what is inside an image?

dew keeps this simple by design: the workflow focuses on packing, restoring, status, and doctor checks rather than a separate image browser.

Use `dew restore --dry-run` to preview what a restore would write, change, or flag as a conflict — without touching the working tree.

## Does `dew restore` overwrite my local files?

Not by default.

`dew restore` is designed to be safe. If a local file already exists and differs from the image, dew reports a conflict and leaves the local file untouched.

To overwrite local differences, you must explicitly pass:

```bash
dew restore --force
```

Use `--dry-run` first if you want to preview:

```bash
dew restore --dry-run
```

## What is `dew hydrate`?

`dew hydrate` is an alias for `dew restore`.

It exists because "hydrate a clone" is the product idea: take a fresh repo clone and restore the missing local context.

These are equivalent:

```bash
dew restore
dew hydrate
```

## When should I run `dew pack`?

Run `dew pack` after you create or change files that dew manages.

Example:

```bash
vim .env.local
dew pack
dew sync
```

Think of `dew pack` as:

> "Capture my current local repo context into the encrypted image."

## When should I run `dew sync`?

Run `dew sync` after `dew pack` when you want to copy the encrypted image to your configured sync destination.

Typical source-machine flow:

```bash
dew pack
dew sync
```

Typical new-machine flow:

```bash
dew sync pull
dew restore
```

## What does `dew doctor` do?

`dew doctor` checks the current repo and tells you what to fix next.

It can detect:

- missing identity
- missing manifest
- empty manifest
- missing image
- image encrypted to a different identity
- corrupt or undecryptable image
- tracked files missing from the working tree

Use it when a clone does not work and you are not sure why:

```bash
dew doctor
```

## What does `dew status` do?

`dew status` gives a quick health summary for the current repo.

It shows whether you have:

- an identity
- a manifest
- an image
- tracked files
- hydration state
- sync destination

Use it as a quick check:

```bash
dew status
```

Use `dew doctor` when you want diagnosis and next steps.

## What does `dew images` do?

`dew images` lists encrypted images managed by dew across all repos.

It runs from anywhere because images live globally under:

```text
~/.dew/images
```

Use it when you want to see what dew has packed on this machine.

## What does `dew clean` do?

`dew clean` removes dew's footprint for the current repo.

It can remove:

- the repo manifest
- the local encrypted image
- or both

Examples:

```bash
dew clean
dew clean --image-only
dew clean --manifest-only
```

It does not remove your global identity key.

## What files should I manage with dew?

Good candidates:

```text
.env.local
.env.development.local
docker-compose.override.yml
certs/
.local/
fixtures/private/
sandbox-config.yaml
dev-notes.md
```

Bad candidates:

```text
node_modules/
dist/
build/
target/
.git/
large database dumps
general downloads
files that should be in Git
production secrets that belong in a secrets manager
```

A good rule:

> If the file is needed to make this repo work locally, should not be committed, and is small enough to carry, it may be a good dew candidate.

## Should I put production secrets in dew?

Usually, no.

dew is primarily for local developer context.

Production secrets should live in production-grade secret management systems with access control, audit, rotation, and deployment integration.

dew can technically package any allow-listed file, but that does not mean every secret belongs in dew.

## Can dew manage large files?

It can, but be careful.

dew compresses and encrypts images, but it is not designed as a large artifact manager or general backup system.

Avoid using dew for:

- database dumps
- videos
- model files
- build artifacts
- dependency directories
- generated outputs

Use Git LFS, artifact storage, object storage, or backups for large files.

## Does dew preserve file permissions?

dew packages files through a tar archive and restores regular files. File permissions are preserved where practical.

However, dew is not a full filesystem snapshot tool. It skips symlinks and special files by design.

If a workflow depends on exact ownership, ACLs, extended attributes, or platform-specific metadata, dew may not be the right layer.

## Does dew follow symlinks?

No.

dew skips symlinks and special files when building the archive. This is intentional. Symlinks can create confusing and unsafe restore behavior.

dew is designed around regular files and directories.

## Can dew restore outside the repo?

No.

Archive extraction is designed to reject absolute paths and path traversal. An image should not be able to write outside the target repo during restore.

This is a core safety behavior.

## What happens if the image was packed from a different repo?

dew uses image ownership metadata to avoid accidental collisions.

If two repos use the same project/image name, dew should avoid overwriting an image created by a different repo unless you explicitly force the operation.

If you see this warning, use a unique project name:

```bash
dew init --project <unique-name>
```

## Can I use dew with private repositories?

Yes.

dew does not care whether the Git repo is public or private. The manifest can be committed to either.

The sensitive file contents live outside the repo in encrypted images.

## Can I use dew with public repositories?

Yes, and that is one of the useful cases.

A public repo can include a `.dew/manifest.yaml` that documents local files needed for development without exposing the private contents.

For example, a public repo might commit:

```yaml
allow:
  - .env.local
  - docker-compose.override.yml
```

But the actual `.env.local` file remains private.

## Does everyone need dew installed to use the repo?

No.

If a repo has a `.dew/manifest.yaml`, developers who do not use dew can ignore it.

The manifest is just metadata. It does not affect normal Git usage.

Developers who want automatic local context restore can install dew.

## How is dew different from Git LFS?

Git LFS is for large files that still belong to the repository.

dew is for local-only files that should not be committed to the repository at all.

Use Git LFS for:

- large assets
- binaries
- datasets that are part of the project

Use dew for:

- private local config
- secrets
- dev certs
- local overrides
- files intentionally ignored by Git

## How is dew different from a dotfiles repo?

A dotfiles repo manages user-level configuration across machines.

dew manages repo-specific local context.

Dotfiles answer:

> "How do I configure my shell/editor/system?"

dew answers:

> "How do I make this cloned repo locally runnable again?"

They are complementary.

## How is dew different from just copying files manually?

Manual copying works until it does not.

Manual copying is easy to forget, hard to audit, inconsistent across machines, and unclear to future-you.

dew gives you:

- an explicit allow-list
- repeatable pack/restore commands
- encryption
- repo-aware metadata
- syncable images
- status and doctor checks

It turns "I think I copied the right files" into a repeatable workflow.

## What is the simplest dew workflow?

On the machine where the repo already works:

```bash
dew keygen
dew init
dew add .env.local
dew add docker-compose.override.yml
dew pack
dew remote set <destination>
dew sync
git add .dew/manifest.yaml
git commit -m "Add dew manifest"
git push
```

On a new machine:

```bash
git clone <repo>
cd <repo>
dew key pull <user@host>
dew remote set <destination>
dew sync pull
dew restore
dew doctor
```

## What should I commit?

Commit:

```text
.dew/manifest.yaml
```

Do not commit:

```text
~/.dew/
~/.dew/identity.age.key
~/.dew/images/*.dew.age
.env.local
private certs
local overrides
```

Usually your `.gitignore` should continue ignoring the actual local files.

## Should `.dew/manifest.yaml` be in `.gitignore`?

No.

The manifest is meant to be committed.

The local encrypted images and identity are stored outside the repo under `~/.dew/`, so they should not appear in normal repo status.

## Can I use dew in CI/CD?

Usually, dew is more useful for local development than CI/CD.

CI/CD should use proper secret injection from the CI system, cloud secret manager, or deployment platform.

That said, some internal workflows might use dew to hydrate test fixtures or local-like environments, but that should be done carefully and intentionally.

## Is dew cross-platform?

dew is designed as a single binary for macOS, Linux, and Windows.

Be aware that path handling, file permissions, SSH availability, and shell behavior can vary by platform. If your team is cross-platform, test your actual workflow on the platforms you support.

## What happens if I run `dew keygen` on a new machine instead of copying my existing key?

You will create a different identity.

That new identity will not be able to decrypt images encrypted to your old identity.

If you are setting up a second machine, do not run `dew keygen` unless you intentionally want a separate identity.

Instead, use:

```bash
dew key pull <user@host>
```

or:

```bash
dew key push <user@host>
```

## Is my sync destination trusted?

You should treat the sync destination as storage for encrypted images.

It does not need your private key. That is the important boundary.

But you should still use a destination you control or trust, because an attacker who can tamper with images can cause denial-of-service or confusion, even if they cannot decrypt the contents.

Use `dew doctor` after pulling if something seems wrong.

## Can dew detect if my local files changed since the last pack?

dew does not compare the working tree against the image — image diffing is intentionally out of scope, and dew keeps no version history to diff against.

The practical habit is to repack whenever you touch a dew-managed file:

```bash
dew pack
dew sync
```

`dew status` will also tell you whether the repo looks hydrated, and `dew restore --dry-run` previews how the image differs from what is on disk.

## Can I have different images for different environments?

Currently, dew is centered around one image per repo/project.

For more advanced environment-specific workflows, you can use different project names or manifests, but be careful. dew is intentionally simple.

If you need formal environment separation for dev/staging/prod secrets, use a real secrets manager.

## Can I share a dew image with someone else?

Only if they also have the private key that can decrypt it.

By default, images are encrypted to your dew identity.

Sharing images and keys should be done carefully. Anyone with the image and private key can read the contents.

For teams, think carefully before sharing a single identity. dew does not provide per-user access controls or revocation — for that, use a real secrets manager.

## What is the security model?

The basic security model is:

- The repo manifest is public/shareable metadata.
- The encrypted image contains private file contents.
- The private key stays on machines you control.
- Sync moves encrypted images, not keys.
- Restore is local and explicit.
- Key transfer is explicit and separate.

dew protects against accidental Git commits of local context by keeping contents outside the repo. It protects synced images with encryption.

dew does not protect against:

- malware on your machine
- someone with access to your private key
- secrets already committed to Git
- weak remote access controls
- lack of key rotation
- production secret governance requirements

## What should I do before using dew with sensitive files?

Use this checklist:

```bash
dew scan
dew add <specific-files>
dew rules
dew pack --dry-run
dew pack
dew restore --dry-run
dew doctor
```

Also review:

```bash
git status
git diff --cached
```

Make sure you are committing only the manifest, not the private files themselves.

## Is dew production-ready?

dew is useful today for local development workflows, especially for solo developers and small teams.

For production secret management, enterprise access control, compliance, auditing, and key rotation, use specialized tools.

A fair way to think about dew:

> Production-grade idea for local developer context. Not a production secrets platform.

## What is the philosophy behind dew?

Git should hold the shared code and shared project truth.

But every working repo also has a local half: private config, dev certs, local overrides, fixture data, and notes that make the clone usable.

That local half is real. It just does not belong in Git.

dew gives that local half a clean, encrypted, repeatable home.
