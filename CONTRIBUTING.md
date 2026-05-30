# Contributing to dew

Thanks for your interest in **dew** — the local-first CLI that manages the private repository state Git intentionally ignores.

> **Project status:** dew is at the design stage. The authoritative MVP spec is [`docs/design.md`](docs/design.md). Implementation has not started yet, so the most valuable contributions right now are spec feedback, design discussion, and helping stand up the initial project structure per the spec.

## Ground rules

- **By contributing, you agree your contributions are licensed under the [Apache License 2.0](LICENSE)**, the same license as the project (see Section 5 of the license).
- Be respectful and constructive. Assume good faith.
- Discuss non-trivial changes in an issue before opening a large PR, so effort isn't wasted on something that's out of scope.

## Scope discipline (read this first)

dew's MVP is **deliberately narrow**. Before proposing a feature, check it against the spec — the following are *explicitly deferred* and PRs adding them will be declined for now:

> version history · snapshot messages · team sharing · multiple recipients · per-repo keys · key rotation/export · cloud provider integrations · GitHub integration · image diffing · conflict resolution · GUI · any server/account system.

dew is **not** a secrets manager, backup tool, Git LFS, or cloud sync service. When in doubt, open an issue and ask.

## What dew is

A local-first Go CLI that packages an allow-listed set of private files (`.env.local`, dev certs, `docker-compose.override.yml`, private fixtures, local config) into a single encrypted image per repo, and can sync that image so a fresh clone can be **hydrated** back to a working state. See the [README](README.md) for the architecture and workflow.

## How to contribute

### Reporting bugs / requesting features

Open an issue with:
- What you expected vs. what happened (for bugs), including OS, dew version, and exact commands.
- A clear use case and how it fits the MVP scope (for features).

### Submitting changes

1. **Fork** the repo and create a branch from `main` (e.g. `feat/pack-command`, `fix/restore-symlink-escape`).
2. Make focused changes — one logical change per PR.
3. Ensure the build and tests pass (see below).
4. Open a PR against `main` with a clear description of *what* and *why*, linking any related issue.

### Commit messages

- Use clear, imperative subject lines (e.g. `Add pack command`, `Reject symlink entries on restore`).
- Keep the subject under ~72 characters; add a body explaining *why* when it isn't obvious.

## Development

dew is written in **Go** and distributed as a single cross-platform binary.

```bash
# Build
go build -o dew .

# Test
go test ./...

# Format & vet (please run before pushing)
gofmt -s -w .
go vet ./...
```

Planned stack: [Cobra](https://github.com/spf13/cobra) (CLI), `gopkg.in/yaml.v3` (config + manifest), `archive/tar`, zstd compression, [age](https://github.com/FiloSottile/age) for encryption, and scp/rsync shell-out for sync.

### Build order

The spec ([`docs/design.md`](docs/design.md) §23) prescribes a deliberate sequence. Please align PRs with it so the core value lands first:

1. Manifest commands (`init`, `add`, `remove`, `list`)
2. Identity (`keygen`, `key status`)
3. Pack / restore — the core value; should work end-to-end before discovery polish
4. Discovery (`scan`, `init --from-gitignore`, `add .`)
5. Health (`status`, `doctor`)
6. Sync (`sync`, `sync pull`)

## Security-sensitive areas

dew handles private files and encryption keys. Two invariants must hold and are reviewed carefully:

- **Path sanitization on restore.** Reject `..` and symlink entries during tar extraction so an image can never write outside the repo (tar-slip / symlink escape).
- **Atomic, non-destructive restore.** Extract to a temp dir then move into place; compare (hash/mtime) before overwriting rather than just warning — with no version history, a careless restore is silent data loss.

If you find a security vulnerability, **please do not open a public issue.** Email the maintainer at barooah.vedanta@gmail.com instead.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
