# dew — CLI help style

How we write `--help` text in dew. These rules produced the help you see today
(the "help v2" pass, PRs #100/#101); follow them for every new or edited command
and flag. The goal: help that orients a newcomer in seconds and tells anyone
exactly what to run next — **without** turning `--help` into the manual.

This is the standard. The exhaustive, edge-case detail belongs in
[`USER-MANUAL.md`](USER-MANUAL.md) and [`COMMANDS.md`](COMMANDS.md), not in
`--help`.

## Principles

1. **Intent before mechanics.** Lead with what the user *accomplishes*, not the
   internal operation. "Bring your local files back from the image" beats
   "Decrypt, decompress, and extract the tar."
2. **A `Long` answers four questions:** *what it does · why you'd run it · what
   happens · what to run next.* If a reader still has one of those after reading,
   the `Long` isn't done.
3. **Reinforce the mental model.** dew carries the local context Git ignores; it
   **complements Git and never touches tracked source.** Echo that framing where
   it fits so each command reinforces the whole.
4. **Calibrated, not verbose.** Match the density of `gh`, `flyctl`, and
   `terraform`. More help is not better help — depth lives in the manual and the
   command reference, linked from the root command.
5. **Tier the depth.** Core commands (`pack`, `restore`/`hydrate`, `sync`, `add`,
   `init`, `doctor`) earn richer `Long`s and multiple examples. Trivial commands
   (`list`, `key status`, `version`) stay terse — a tight `Short` and one example.

## Mechanics

- **`Short`** — one verb-first outcome (what you get, not what it is), ≤ ~60
  chars, **no trailing period**. Surface aliases inline: `"Stop tracking a path
  (alias: rm)"`. This is what shows in `dew --help`, so it must read as a list.
- **`Long`** — 1–2 short paragraphs, **hard-wrapped at ~78 columns**. Name the
  next command in single quotes so it's copy-pasteable: `'dew pack'`,
  `'dew sync pull'`.
- **`Example`** — progressive, simplest → most powerful, each with an inline
  `# comment` that says *why*:

  ```
  dew restore             # write the tracked files back into the repo
  dew restore --dry-run   # preview new / unchanged / conflicts; change nothing
  dew restore --force     # overwrite local files that differ from the image
  ```

- **Flags** — outcome-focused wording: `"preview what would change; touch
  nothing"`, not `"enable dry-run mode"`.
- **Root command** — group subcommands by purpose (Identity / Repository / Image
  / Sync / Health & inventory), and point to per-command `--help` and the guide
  URL.

## Process

- **Verify by rendering**, not by reading source: build the binary and eyeball
  the actual `dew <command> --help` output (and `dew --help`) before committing.
  Confirm `Long`s wrap cleanly and the grouped list reads well.
- Keep docs in step: when help changes, check whether
  [`USER-MANUAL.md`](USER-MANUAL.md) / [`COMMANDS.md`](COMMANDS.md) need the same
  update.
- Help-text-only changes are still PRs through CI like any other change.
