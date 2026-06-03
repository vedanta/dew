# dew — Step-by-step guide

A task-first walkthrough of **dew**, organized by what you're trying to do. Each
step is a command you can copy, with a one-line note on what it does.

> **The one idea:** Git tracks your project's *shared* code; dew manages the
> *private, local* context it can't hold — `.env.local` and secrets, dev certs,
> overrides, machine-specific config, the local notes you don't commit — as a
> single **encrypted image** per repo. dew complements Git: shared docs still
> live in Git, and dew never touches your tracked source.

**Pick your scenario:**
1. [One machine](#scenario-1--one-machine) — track and snapshot your local files.
2. [Two machines](#scenario-2--two-machines) — set your local files up on a second machine.
3. [More machines](#scenario-3--more-machines-advanced) — a few of your own machines (advanced).

For the full reference see the [user manual](USER-MANUAL.md) and
[command reference](COMMANDS.md).

---

## Before you start

**Install dew** (once per machine):

```bash
brew install --cask vedanta/dew/dew        # macOS
# or download a binary: https://github.com/vedanta/dew/releases/latest
dew version                                # confirm it's installed
```

**Create your identity** (once per machine — this is the key that encrypts and
decrypts your images):

```bash
dew keygen          # writes ~/.dew/identity.age.key (guard it like any private key)
dew key status      # shows your public key
```

> Your identity is **yours**, shared across all your repos. On a *new* machine you
> bring this same key over (Scenario 2) — you don't run `keygen` again.

---

## Scenario 1 — One machine

**Goal:** capture the gitignored files a repo needs into one encrypted image —
so you can restore them after an accidental delete, or get ready to go
multi-machine.

```bash
cd my-app

dew init                              # 1. create .dew/manifest.yaml in the repo
dew scan                              # 2. see which local files dew suggests tracking
dew add .env.local certs/             # 3. track specific files (or 'dew add .' for the suggestions)
dew list                              # 4. review what's tracked  (use 'dew rules' to see deny rules too)
dew pack                              # 5. build the encrypted image at ~/.dew/images/my-app.dew.age

git add .dew/manifest.yaml            # 6. commit the manifest (it lists paths only — no secrets)
git commit -m "Add dew manifest"
```

After you edit a tracked file, just re-run `dew pack` — you declare files once
with `dew add` and never re-add. To bring the files back (e.g. after deleting
one):

```bash
dew restore                           # writes the tracked files back into the repo
dew restore --dry-run                 # preview first — change nothing
dew doctor                            # checks the repo and reports the next step
```

> **Note:** on a single machine there's no key to move. dew here is a repo-aware,
> encrypted snapshot of your local files — not a backup tool (no history; one
> image per repo, latest pack only).

---

## Scenario 2 — Two machines

**Goal:** you packed on machine **A**; now hydrate the same local files on machine
**B** (a new laptop, a dev server like `nvk2`, …).

### On machine A (the source) — publish

```bash
# (in the repo, identity + manifest already set up as in Scenario 1)
dew remote set nas:/volume1/dew       # 1. where images sync — a local path or scp-style host:path
dew remote test                       # 2. check it's reachable & writable
dew sync                              # 3. push the encrypted image to the destination

git push                              # 4. push the repo (with the committed .dew/manifest.yaml)
```

### Bring your identity to machine B (one-time)

dew never moves your key automatically — you provision it explicitly, over SSH,
to a machine you control:

```bash
# from machine A, push your identity to B:
dew key push you@machineB
# ── or, run this ON machine B to pull it from A: ──
dew key pull you@machineA
```

> ⚠️ **Don't run `dew keygen` on machine B.** That creates a *different* identity
> that can't decrypt your images. Bring the existing one over (above), or copy
> `~/.dew/identity.age.key` by hand (the `.pub` is optional — dew derives it).

### On machine B (the target) — hydrate

```bash
# install dew first (see "Before you start"); your identity is already here from the step above
git clone <repo> && cd my-app         # 1. the clone already has .dew/manifest.yaml
dew remote set nas:/volume1/dew       # 2. point at the same destination
dew sync pull                         # 3. fetch the encrypted image
dew restore                           # 4. write your local files into the repo
dew doctor                            # 5. → Repository fully hydrated.
```

That's it — machine B now has the same working tree as A.

> **If `dew restore` says "encrypted to a different identity":** the key on B
> doesn't match the one that packed the image. Bring machine A's
> `~/.dew/identity.age.key` over (via `dew key push`/`pull`), and don't `keygen`
> on B.

---

## Scenario 3 — More machines (advanced)

**Goal:** use dew across a few of *your own* trusted machines (laptop, desktop, a
server).

Each new machine is just Scenario 2's bootstrap again — provision the identity,
then `clone → remote set → sync pull → restore`:

```bash
dew key push you@machine3             # provision each new machine…
dew key push you@machine4
dew key devices                       # …and see where your identity has been sent/received
```

```text
PEER             DIRECTION       FINGERPRINT        WHEN                  LABEL
you@machine3     sent-to         age1xdde…          2026-06-03T00:06:31Z  -
you@machine4     sent-to         age1xdde…          2026-06-03T00:10:02Z  -
```

> ⚠️ **Use with care — this is beyond dew's core scope.** All your machines share
> **one** identity, and dew has **no revocation or key rotation**: if a machine is
> lost, that shared key is compromised everywhere, and `dew key devices` is a
> best-effort *log*, not a way to revoke a machine. Fine for a handful of machines
> you control; not for teams or disposable/untrusted machines.

---

## Quick reference

| Do this | Command |
|---|---|
| Create your identity (once per machine) | `dew keygen` |
| Set up a repo | `dew init` |
| See / choose local files to track | `dew scan` · `dew add <path>` · `dew add .` |
| Review what's tracked / why | `dew list` · `dew rules` |
| Build the encrypted image | `dew pack` (`--dry-run` to preview) |
| Set / check the sync destination | `dew remote set <dest>` · `dew remote test` |
| Push / fetch the image | `dew sync` · `dew sync pull` |
| Restore your local files | `dew restore` (alias `dew hydrate`; `--dry-run` to preview) |
| Move your identity to another machine | `dew key push <user@host>` · `dew key pull <user@host>` |
| See where your identity has gone | `dew key devices` |
| Check health / next step | `dew status` · `dew doctor` |
| List all images you manage | `dew images` · `dew remote images` |

## Troubleshooting

| Symptom | Fix |
|---|---|
| `no identity found` | `dew keygen` (or bring your key over with `dew key push`/`pull`) |
| `no manifest found` | `dew init` |
| `encrypted to a different identity` | Bring the *same* key the image was packed with; don't `keygen` on a new machine |
| `Hydration: Incomplete` / files missing | `dew sync pull` then `dew restore` |
| `no destination configured` | `dew remote set <dest>` |
| `required tool "scp"/"ssh" not found` | Install OpenSSH, or use a local/mounted destination |

Run **`dew doctor`** first whenever a clone isn't working — it names the one
thing to fix next.
