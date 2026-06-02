# dew — Build Log

A record of how the dew MVP was built: the phased plan, the engineering
workflow, key design decisions (and why), and the per-phase PRs. The
authoritative spec is [`design.md`](design.md); the operational plan is
[`build-plan.md`](build-plan.md). This log is the narrative of executing it.

## At a glance

- **Language/stack:** Go single binary — Cobra, `gopkg.in/yaml.v3`, native [`filippo.io/age`](https://filippo.io/age), pure-Go [`klauspost/compress/zstd`](https://github.com/klauspost/compress), `sabhiram/go-gitignore`.
- **Scope:** all 27 sub-phase issues across 7 milestones, plus one gap-fix (per-manifest deny).
- **~33 PRs**, every one merged green through CI (lint + 3-OS unit matrix + 2-OS acceptance) under branch protection.
- **~2,300 lines** of non-test Go and **~2,200 lines** of test Go; **93 unit test functions** + **7 acceptance scripts**.
- **External runtime deps:** none for crypto/compression; `scp` only when syncing to a remote `host:path`.

## Engineering workflow

Established before any feature code (Phase 0) and held for the whole build:

- **Phase → GitHub milestone, sub-phase → issue, sub-phase → one PR → `main`.** Flat branching; `main` always shippable.
- **CI gates every PR** and is required by branch protection: `gofmt -s`, `go vet`, `golangci-lint` (v2, incl. `gosec`), `go test -race` on ubuntu/macos/windows, and the acceptance suite (binary-level tests) on ubuntu/macos.
- **Two test layers per command:** Go unit tests (logic, via testable `do*` helpers that take an `io.Writer`/paths so they need no real `$HOME`) and shell acceptance scripts that drive the built binary in an isolated `$HOME`.
- **Self-merge allowed** (no required reviews) but checks are mandatory even for admins.

The 3-OS matrix repeatedly earned its place — it caught three real Windows portability bugs before they reached `main` (see Lessons).

## Phase 0 — Setup

Skeleton + test harness + CI, built first so every later phase plugged in.

- **#29 (0.1)** scaffold: `go.mod`, Cobra root, full 13-command surface as stubs, `internal/` package stubs, `Makefile`.
- **#28 (0.2)** CI workflow, `.golangci.yml`, Dependabot; branch protection enabled afterward.
- **#34 (0.3)** acceptance harness (`run.sh`, `lib.sh`, smoke test).
- Housekeeping: **#33** (Dependabot action bumps consolidated), **#35/#36/#38** (logo → `assets/`, flow banner, README).

## Phase 1 — Repo & Manifest

The manifest is the repo-level contract (`.dew/manifest.yaml`).

- **#37 (1.1)** `manifest` package — schema, validation, load/save.
- **#39 (1.2)** `dew init` — refuses to clobber; `--from-gitignore` wired (discovery deferred to Phase 4).
- **#40 (1.3)** `dew add` — dedupe + path safety (`repoRelPath` rejects `..`/outside-repo).
- **#41 (1.4)** `dew remove`, **#42 (1.5)** `dew list`.

## Phase 2 — Identity

- **#43 (2.1)** `identity` package. **Key decision:** resolved design §12 in favor of the **native age library over shelling out** to `age-keygen` — keeps dew a single self-contained binary and makes tests cross-platform with no CI installs.
- **#44 (2.2)** `dew keygen` (refuses overwrite, `0600` key), **#45 (2.3)** `dew key status`.

## Phase 3 — Pack & Restore (the core value)

The heaviest phase; carries the security invariants.

- **#46 (3.1)** `archive` — tar build/extract with **tar-slip + symlink-escape rejection**.
- **#47 (3.2)** `crypto` — native age encrypt/decrypt (streaming `EncryptWriter`/`DecryptReader`).
- **#48 (3.3)** `compress` — pure-Go zstd.
- **#49 (3.4)** `dew pack` — `tar → zstd → age` streamed to `~/.dew/images/<project>.dew.age`, written atomically (temp + rename).
- **#50 (3.5)** `restore` package — **atomic, non-destructive**: stage to temp, sha256-compare, never overwrite a diverged file without `--force`.
- **#51 (3.6)** `dew restore` + a byte-for-byte pack→wipe→restore round-trip test.

## Phase 4 — Discovery

- **#52 (4.1)** `scanner` — `.gitignore` + tree walk → candidates vs noise.
- **#53 (4.2)** `dew scan`, **#54 (4.3)** `dew init --from-gitignore` (seeds the allow-list), **#56 (4.4)** `dew add .` (interactive, never "everything").
- **#58 (gap-fix)** wired up **per-manifest `deny:`**, which had been an inert schema field. Introduced `internal/deny` (built-in + per-manifest patterns) consumed by both `scanner` and `pack` (an allow-listed directory never packs noise). Resolved part of design §27.

## Phase 5 — Health

- **#59 (5.1)** `dew status` — identity/manifest/image/tracked/hydration summary.
- **#60 (5.2)** `dew doctor` — prioritized diagnosis + single recommended next action; verifies the image actually **decrypts** (not just that it exists).

## Phase 6 — Sync

- **#61 (6.1)** `config` package (`~/.dew/config.yaml`, sync destination).
- **#62 (6.2)** `sync` package. **Key decision:** **hybrid transport** — pure-Go atomic copy for local/mounted destinations (zero deps), `scp` shell-out for remote `host:path`. Chosen because shelling out **inherits the user's `~/.ssh/config`, agent, and `known_hosts`** — far less configuration overhead and it **offloads auth + host-key security to OpenSSH** rather than dew reimplementing it. `depcheck.RequireTool` makes the `scp` dependency a graceful, hint-bearing error, and only for remote destinations.
- **#63 (6.3)** `dew sync` (push), **#64 (6.4)** `dew sync pull` + the full-hydrate capstone (clone → pull → restore → doctor = healthy). The last stub helper was removed here — every command is now real.

## Key decisions (and why)

1. **Native age, not the `age` CLI** (§12) — single binary, no external crypto dep, portable tests.
2. **Pure-Go zstd** — same reasoning; no external `zstd`.
3. **Hybrid sync** (§14) — local copy (no deps) + `scp` for remote. Trades binary purity for *less* config and *better* security by delegating SSH to the system.
4. **`do*` helper pattern** — command logic lives in pure functions taking `(root, paths…, io.Writer)`, so unit tests never touch the real `~/.dew` and prompts/output are capturable.
5. **`$DEW_HOME` override** — lets tests and power users relocate `~/.dew`.

## Lessons / things the matrix and linters caught

- **Windows has no Unix permission bits** — a `0600` assertion had to be skipped on Windows (identity test).
- **`/etc/passwd` isn't absolute on Windows** — a path-safety test used a non-portable absolute path; fixed to compute one from the repo's parent.
- **`filepath.HasPrefix` is deprecated** — staticcheck flagged it; switched to `strings.HasPrefix`.
- **gosec is strict on purpose** — file-mode preservation on extract, variable file paths, and tar/zstd copies needed deliberate handling (mask via `FileInfo().Mode()`, justified `//nolint` where the input is the user's own data).
- **`fmt.Fprintf` to an `io.Writer` is errcheck-flagged** (unlike to a `strings.Builder`) — buffered output where possible; added a small `outf` helper for interactive prompts.
- **Transient CI flake** — `brew install age` once failed on a macOS runner (`ghcr.io` DNS); this fed directly into deferring, then eliminating, the `age` install once crypto went native.

## Outcome

The defining workflow works end-to-end across machines:

```
source machine:  dew keygen → dew init → dew add … → dew pack → dew sync
new machine:      dew sync pull → dew restore → dew doctor  ⇒  Repository fully hydrated.
```

## Post-MVP work

After the MVP, several hardening and UX features shipped (each its own CI-gated
PR, same workflow):

- **Per-manifest deny wired up** — the inert `deny:` field made functional via a
  shared `internal/deny` matcher (built-in + per-manifest), consumed by both
  discovery and `pack`.
- **`dew init --project`** — name a project independently of its folder, with
  path-safe validation and a collision warning.
- **Pack-time repo-binding** — a committed manifest `id` + per-image ownership
  marker; `pack` refuses to overwrite an image created by a different repo
  (unless `--force`), closing the silent cross-repo clobber.
- **`--dry-run`** for `pack` and `restore` — preview without writing (a shared
  `archive.eachFile` walk powers both `Build` and a new `List`).
- **`dew hydrate`** — alias for `restore` (the product's core verb).
- **Global deny layer + `dew rules`** — a third (user-level) deny layer in
  `~/.dew/config.yaml`, and an inspection command showing all three layers.
- **`dew images`** — global image inventory (project, size, last-packed, owner),
  repo-independent.

A 37-assertion end-to-end test (two simulated machines, shared remote, manual
key transfer, wrong-key detection, deny exclusion, non-destructive restore +
`--dry-run`/`--force`, cross-repo collision guard) passes clean.

## Releases & distribution

Distribution was added after the MVP, each piece its own CI-gated PR:

- **Release pipeline** — [GoReleaser](https://goreleaser.com) builds six targets
  (linux/macOS/windows × amd64/arm64), generates checksums, and publishes a
  GitHub Release. Triggered purely by pushing a `v*` tag (`release.yml`); the
  version/commit/date are injected via ldflags and surfaced by `dew version`.
- **Homebrew** — a `homebrew_casks` block updates the
  [`vedanta/homebrew-dew`](https://github.com/vedanta/homebrew-dew) tap on each
  release (`brew install vedanta/dew/dew`). macOS-primary, with Linux URLs too.
  (Switched from the deprecated `brews` block; macOS binaries are not yet
  notarized.)
- **Product site** — a static landing page deployed to
  [vedanta.github.io/dew](https://vedanta.github.io/dew/) via GitHub Pages
  (`pages.yml`), independent of the binary.

Release history:

- **v0.1.0** — first tagged release; established the pipeline, the Homebrew cask,
  and version stamping end-to-end (verified via a `brew install` smoke test).
- **v0.2.0** — `dew hydrate` promoted from a hidden alias to a first-class listed
  command; `ls`/`rm` aliases surfaced in help; **CLI help v2** — every command's
  `--help` rewritten to a calibrated, self-contained style (intent before
  mechanics: what it does, why, what happens, what to run next). Docs (user
  manual + command reference) aligned. No functional fixes or breaking changes —
  a clean additive/polish minor.
- **v0.3.0** — the **`dew remote`** command family: the sync destination is now
  CLI-managed instead of a hand-edited `~/.dew/config.yaml`. `dew remote set` /
  `dew remote` / `dew remote unset` configure, show, and clear it; `dew remote
  test` checks a destination is reachable, trusted, and writable (over `ssh` for
  remotes, surfacing OpenSSH's verdict); `dew remote images` lists what's stored
  there. `dew status` now shows the real destination, and `dew sync`'s
  no-destination error points at `dew remote set`. Adds an `ssh` dependency for
  remote `test`/`images` only (ships with the `scp` already required; local needs
  nothing). Tracked under #111 (children #108–#110); also shipped a product-site
  rewrite and a Node-24 CI bump. No breaking changes — a new command surface.

Remaining backlog is tracked in GitHub issues (e.g. `dew images` repo-locations,
discover+pack convenience).

Full docs set: [`design.md`](design.md) (spec), [`build-plan.md`](build-plan.md)
(plan), this log, [`USER-MANUAL.md`](USER-MANUAL.md), [`COMMANDS.md`](COMMANDS.md),
and [`manual-test-plan.md`](manual-test-plan.md).
