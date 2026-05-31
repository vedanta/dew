# dew — Build Plan

This plan operationalizes the build order in [`design.md`](design.md) §23 into a
trackable GitHub workflow.

## Conventions

- **Phase → GitHub Milestone.** Each phase is a milestone; closing it means the
  phase is demoable end-to-end.
- **Sub-phase → GitHub Issue.** Each sub-phase is one issue with explicit
  acceptance criteria.
- **Sub-phase → one PR → `main`.** Flat branching (`feat/<sub-phase> → main`).
  No long-lived phase branches. `main` stays green and shippable at all times.
  Each PR uses `Closes #N` to auto-close its issue.
- **CI gates every PR.** A PR cannot merge unless CI is green:
  - `gofmt -l` (no diffs), `go vet ./...`, `golangci-lint run`
  - `go test ./...` (unit, table-driven)
  - `./test/acceptance/run.sh` (shell scripts driving the built `dew` binary)
- **Branch protection on `main`:** require PR + passing CI; no direct pushes.
- **Definition of Done (every sub-phase):** code + unit tests + acceptance
  coverage where it touches the binary boundary + docs/README updated if
  user-facing behavior changed.

## Phase 0 — Setup (Milestone 0)

Establish the skeleton and the test harness *first* so every later sub-phase
plugs into it.

| Sub-phase | Issue | Deliverables | Tests / acceptance |
|---|---|---|---|
| 0.1 | Project scaffolding | `go mod init`, Cobra root command, `cmd/` + `internal/` package stubs per design §22, `dew --version`/`--help` | `dew --help` runs; `go build` produces a binary |
| 0.2 | CI pipeline & branch protection | GitHub Actions: build, gofmt, vet, golangci-lint, `go test`, acceptance runner; protect `main` | CI green on a no-op PR; direct push to `main` rejected |
| 0.3 | Acceptance test harness | `test/acceptance/` layout, helper lib (temp repo, temp `$HOME`, run binary), one smoke test | `./test/acceptance/run.sh` passes in CI |

## Phase 1 — Repo & Manifest (Milestone 1)

| Sub-phase | Issue | Deliverables | Tests / acceptance |
|---|---|---|---|
| 1.1 | `manifest` package | read/write `.dew/manifest.yaml`, schema (version, project, image, allow, deny), validation | unit: round-trip, malformed-yaml, missing-fields |
| 1.2 | `dew init` | create `.dew/manifest.yaml`; `--from-gitignore` flag wired (discovery stubbed until Phase 4) | acceptance: init in temp repo → valid manifest |
| 1.3 | `dew add <path>` / `add <dir>` | append to allow-list, dedupe, reject paths outside repo | unit + acceptance: add file & dir, idempotent |
| 1.4 | `dew remove <path>` | remove from allow-list | unit + acceptance: remove existing & absent path |
| 1.5 | `dew list` | print tracked files | acceptance: list reflects add/remove |

## Phase 2 — Identity (Milestone 2)

| Sub-phase | Issue | Deliverables | Tests / acceptance |
|---|---|---|---|
| 2.1 | `identity` package | age keypair gen, create `~/.dew/` + `~/.dew/images/` if missing | unit: key files written, perms `0600` on private key |
| 2.2 | `dew keygen` | generate identity; **refuse to overwrite** an existing one | acceptance: first run creates; second run errors, leaves key intact |
| 2.3 | `dew key status` | report presence + public key | acceptance: present / absent states |

## Phase 3 — Pack & Restore (Milestone 3) — core value

| Sub-phase | Issue | Deliverables | Tests / acceptance |
|---|---|---|---|
| 3.1 | `archive` package | tar build/extract **with path sanitization**: reject `..` and symlink entries (tar-slip / symlink escape) | unit: malicious tar (`../escape`, symlink) is rejected |
| 3.2 | `crypto` package | age encrypt/decrypt via the native `filippo.io/age` library (no external `age` CLI dependency — resolves design §12) | unit: encrypt→decrypt round-trip; wrong-key fails |
| 3.3 | compression | zstd compress/decompress wiring | unit: round-trip integrity |
| 3.4 | `dew pack` | manifest → tar → zstd → age → `~/.dew/images/<project>.dew.age`; validate identity + allowed files exist | acceptance: pack writes image; missing file errors clearly |
| 3.5 | `restore` package | **atomic, non-destructive**: extract to temp dir, hash/mtime compare, warn/skip before overwrite, then move into place | unit: no overwrite of newer/differing file without consent |
| 3.6 | `dew restore` + round-trip | extract into repo preserving perms | **acceptance: pack → wipe → restore reproduces files byte-for-byte** |

## Phase 4 — Discovery (Milestone 4)

| Sub-phase | Issue | Deliverables | Tests / acceptance |
|---|---|---|---|
| 4.1 | `scanner` package | parse `.gitignore` + walk tree; built-in deny patterns (`node_modules/`, `dist/`, `build/`, `target/`, `.venv/`, `__pycache__/`, `.DS_Store`, `*.log`) | unit: candidates vs skipped classification |
| 4.2 | `dew scan` | print candidates + skipped | acceptance: noise excluded, real candidates shown |
| 4.3 | `dew init --from-gitignore` | complete the flag: init + run discovery | acceptance: manifest seeded from suggestions |
| 4.4 | `dew add .` | add **discovered candidates** (not all repo files), interactive Y/n | acceptance: `add .` never sweeps in deny-listed paths |

## Phase 5 — Health (Milestone 5)

| Sub-phase | Issue | Deliverables | Tests / acceptance |
|---|---|---|---|
| 5.1 | `dew status` | identity / manifest / image / tracked count / hydration / sync summary | acceptance: healthy vs degraded states |
| 5.2 | `dew doctor` | diagnose (missing identity/manifest/image/files, undecryptable image, no sync dest, **missing `scp`/`rsync` for sync**) + recommend next action | acceptance: each problem → correct recommendation |

## Phase 6 — Sync (Milestone 6)

| Sub-phase | Issue | Deliverables | Tests / acceptance |
|---|---|---|---|
| 6.1 | `config` package | read/write `~/.dew/config.yaml`, sync destination | unit: round-trip, missing config |
| 6.2 | `sync` package | scp/rsync shell-out; **images only, never keys**. Preflight: `depcheck.RequireTool` gracefully errors if `scp`/`rsync` is absent (dew's only external runtime dep — crypto is native, see §3.2) | unit: command construction; guard rejects key paths; missing-tool error |
| 6.3 | `dew sync` (push) | copy current image to destination | acceptance (local dir as dest): image appears at dest |
| 6.4 | `dew sync pull` | copy image from destination into `~/.dew/images/` | **acceptance: full hydrate — clone → sync pull → restore → doctor = healthy** |

## Acceptance-test layout

```
test/acceptance/
├── run.sh            # runs all *.sh, fails fast, used by CI
├── lib.sh            # helpers: mk_temp_repo, mk_temp_home, run_dew, assert_*
├── 01_manifest.sh
├── 02_identity.sh
├── 03_packrestore.sh   # the round-trip + tar-slip/symlink security cases
├── 04_discovery.sh
├── 05_health.sh
└── 06_sync.sh          # local dir as sync destination
```

Each script runs against the freshly built binary with an isolated `$HOME` and a
throwaway temp repo, so tests never touch the developer's real `~/.dew/`.

## Runtime dependencies

dew aims to be a single self-contained binary. Crypto (keygen, pack, restore)
uses the native `filippo.io/age` library, so there is **no external `age`
dependency**. The only external runtime tools are `scp`/`rsync`, used solely by
`dew sync`. Commands check the tools they actually need via
`depcheck.RequireTool` and exit gracefully with an install hint if one is
missing — e.g. `dew list` never fails because `scp` is absent. `dew doctor`
reports availability proactively.

## Cross-phase invariants (reviewed on every relevant PR)

- Restore never writes outside the repo (`..` / symlink entries rejected).
- Restore never silently destroys data (compare before overwrite; no version history).
- Sync transfers encrypted images only — never the private key.
- `pack` only includes manifest allow-list paths — never "everything ignored."
- Commands preflight only their own external deps and exit gracefully if missing.
