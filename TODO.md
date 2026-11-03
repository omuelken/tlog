# TODO: make the command line do everything

**Done on 2026-11-03**, the same day, except where a line below says
otherwise. Kept as the record of what was found; `DECISIONS.md` has the
reasoning for the choices that were not obvious (exit 3 for a missing page,
exit 0 with a warning for a save whose commit failed, undo through git).

An audit of `tlog` on 2026-11-03 asked one question: can a script or an agent
do everything a person can do in the outliner or the desktop app? `AGENTS.md`
says anything only an adapter can do is a bug, and the CLI is the adapter
agents use. Most block editing is covered (`tlog block`), but some things are
missing and some commands are hard to use from a script.

Ordered by importance. Each item names the command or flag it should become,
following the existing conventions: `-json` output carrying addresses,
`tlog block <addr> <op>` for anything done to a block, exit 3 when the move
does not exist, exit 4 when the file moved.

## 1. Safety: writes that skip the hash check

These break the rule `AGENTS.md` cares about most.

- [x] **An empty hash skips the check.** `ParseAddr` accepts
  `journals/X.md:0@`, and `Resolve` skips the comparison when the hash is
  empty (`internal/app/service.go`, in `Resolve`). `tlog block
  "journals/X.md:0@" text …` overwrote the block and exited 0. Refuse an empty
  hash in `ParseAddr` (exit 2).
- [x] **Addresses can leave the notes directory.** `tlog view ../out.md -json`
  read a file outside it, and `tlog block ../out.md:0@<hash> text …` rewrote it.
  `Store.Abs` is a plain `filepath.Join`. Refuse in `Store` any path that
  escapes the root, so every caller is covered.
- [x] **Flags written after the text become text.** Only search, view, index,
  files, due and todo use `parseFlags`. The others stop parsing at the first
  word:
  - `tlog block A text "new" -json` wrote "new -json" into the note;
  - `tlog add "buy milk" -todo` wrote "buy milk -todo";
  - `tlog attach f.txt -link-only` attached the file, then failed with exit 1,
    leaving half a result.

  `parseFlags` everywhere now.

## 2. Missing commands

- [x] **Slash commands have no CLI.** This is the largest gap. An agent cannot:
  - set a deadline the way `/deadline` does (makes the block a task, writes the
    configured property name, replaces any other spelling);
  - mark a task done or open without toggling it;
  - quote a block;
  - add a page alias;
  - insert a date or file link.

  Add `tlog block <addr> cmd <name> [arg] [-json]` over `RunCommand`. Better, a
  core `ApplyCommand(addr, name, arg)` that needs no text or caret span. Add
  plain ops for the common ones: `tlog block <addr> done|open|deadline
  <date>|quote` and `tlog alias <page> <name>`, plus `tlog commands -json` to
  list them. An unreadable date exits 3.
- [x] **`tlog add` prints a path, not an address.** An agent has to view the
  page again and guess which block it just wrote. Add `-json` returning
  `{addr, rel, offset, hash}`; the core `Add*` functions need to return a
  `*Result`.
- [x] **`add -todo` only writes to today's journal.** The CLI refuses `-p` with
  it. Allow `tlog add -p Page -todo -due fri`.
- [x] **No date resolution.** Add `tlog date <shorthand> [-n N] [-json]` over
  `DateChoices`, so an agent can check that "fri" means 2026-11-06 before
  writing it. That check is why dates are refused rather than guessed.
- [x] **No list of people.** Add `tlog people [-json]` (`Lookup.People`) and
  `tlog mentions <query> -json` (`Lookup.Mentions`), so an agent writes
  `[[Julia Rausenberger]]` and does not create a new page by misspelling her.
- [x] **No link resolution.** Add `tlog links <addr> -json`: the block's page
  links (resolved to rel and address) and its web and file links (resolved
  with `ResolveLink`; exit 3 if refused). Opening them can stay interactive.
- [x] **No undo for scripts.** `Service.Undo` keeps its history in memory for
  one process, so a one-shot command can never use it. Either add
  `tlog undo [rel]` backed by git (revert the file's last tlog commit if the
  file still matches it, otherwise exit 4), or say in the help that scripts
  undo through git.
- [x] **Small block operations missing:**
  - `tlog block <addr> split -child` (the TUI and the window can, the CLI
    cannot);
  - `tlog append <rel> -if-match <hash> text` over `AppendBlock`;
  - read the text from stdin for `block text|after|before` when the argument
    is `-`, for multi-line blocks.
- [x] **Read-only gaps:**
  - `tlog sync -json` (`Sync`, including the last push error);
  - `tlog places <query> -json`;
  - `tlog config -json`, and `tlog config <key>` to read one value;
  - relative days for `view`: `-day yesterday`, `-day -1`;
  - `tlog block <addr>` with no operation, to read one block.

## 3. Commands that exist but are hard to use from a script

- [x] **Exit codes are inconsistent.**
  - "no block at offset N" (right hash, wrong offset) exits 1; it should be 3.
  - `fail` checks for `ErrConflict` with a type assertion rather than
    `errors.As`, so a wrapped conflict (from undo) exits 1 instead of 4.
  - `view pages/Missing.md` exits 0 with an empty page, but `view Missing`
    exits 1.
  - A commit or push failure after a successful write is only printed to
    stderr, with exit 0, and is not in the JSON. **Now in the JSON** as
    `warning`; still exit 0, because the note was saved.
- [x] **Empty lists print `null`.** `search -json` and `due -json` print
  `null`; `todo` fixes this in the adapter. The core should return empty
  slices, so every command prints `[]`.
- [x] **`search -json` returns only each block's first line.** Multi-line
  blocks are cut off with nothing to say so. Return the full `text`, with
  `firstLine` as a separate field.
- [x] **`block … ref` returns only the link.** Naming a block rewrites its file,
  so the caller's address is stale afterwards and nothing says what it became.
  Return `{link, addr, hash}`.
- [x] **`task` toggles.** Run twice by a retrying agent, it puts the task back.
  The help also gets it wrong: it says plain → open → done → plain, but done
  goes back to open. Fix the help; the `done`/`open` ops above are the real fix.
- [x] **`index -json` has names but no paths.** `Places` already has them.
- [x] **Plain `view` drops properties and deadlines.** Only `-json` shows them.
- [x] **`block <addr> after` with no text** silently inserts an empty block;
  refuse it, or say so.
- [x] **The usage text is stale.** It is missing:
  - `attach -p` and `-link-only`;
  - `files -json` and `-links`;
  - `due -json`, `index -json`, `search -n`;
  - exit code 3 in the `block` help;
  - that `-dir` must come before the subcommand: `tlog view -dir X` silently
    reads "-dir X" as a page and exits 0.
- [x] **`push` says the wrong things.** `push -auto on` prints the notes
  directory where it means the remote. With no remote, it suggests
  `git remote add` although `tlog config git.remote <url>` exists.
- [x] A stale comment above `cmdConfig` mentions a `settable` list that no
  longer exists.

## 4. Logic in the CLI that belongs in `internal/app`

- [x] **Pushing.** `cmdPush` does its own no-remote check, Commit, Push, and
  its own on/off parsing, duplicating the window's `API.Push`. Make a core
  `PushNow()`, and have `SetAutoPush` use `parseBool` from `settings.go`.
- [x] **Reading a page by name without creating it.** `resolveForReading`
  (path or name, aliases, "does not exist") exists only in the CLI. The window's
  `ViewPage` creates the page instead. Move it into the core.
- [x] **The JSON shape of a write.** `cmdBlock` builds
  `{addr, rel, offset, hash}` by hand because `app.Result` has no JSON tags and
  no `Addr` field. Give it both. The nil → `[]` fix for `todo` belongs in the
  core too.
- [x] **`AddTodo` ignores the `deadline.property` setting.** It always writes
  `Deadline::`, while `/deadline` uses `s.Cfg.DeadlineProperty()`. With the
  setting at `due`, `add -todo -due tomorrow` still wrote `Deadline::`. This is
  a bug, not just tidiness.
- [x] **Outside the CLI, the same rule** (two of three):
  - the outliner's Ctrl+P builds its own journal and page list instead of using
    `Lookup.Places`;
  - the outliner hides only `DeadlineProp` beside a block, not the configured or
    alternative spellings;
  - the window's `DatePreview` and `Month` assemble `dates.*` themselves
    (presentational, minor). **Left as is**: it is layout, not logic.
