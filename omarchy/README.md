# tlog in the Omarchy bar

A bar widget for [Omarchy](https://omarchy.org): every open tlog todo, one
click away, coloured by what is most pressing.

- **The pill** is a notebook with how many todos are open. It turns the theme's urgent
  colour when something is overdue and its accent when something is due today.
- **Click** for the list: overdue, today, this week, later, someday. Click a
  circle (or press space) to tick a todo off; click the row (or enter) to open
  its page in the outliner.
- **Capture**: type a todo and enter; it lands in today's journal. End it with
  `@fri`, `@tomorrow`, `@12.11` or `@in 3 days` for a deadline — tlog parses it,
  and refuses what it cannot read. Shift+enter adds a plain note instead.
- **Right-click** opens today's journal (or focuses it, if the outliner is
  already open); **middle-click** opens straight into capture.

Keys in the panel: ↑↓ or j k choose, space or x tick off, enter open, a add,
t journal, r refresh, esc close.

It reads with `tlog todo -json` and writes with `tlog block <addr> task` and
`tlog add -todo`, so it never parses or writes a note itself; a todo changed
since it was listed is refused by tlog rather than ticked off by mistake. It
refreshes whenever today's journal changes on disk, and once a minute.

## Install

```sh
ln -s ~/src/tlog/omarchy ~/.config/omarchy/plugins/io.github.tilman-schieber.tlog
omarchy-shell shell rescanPlugins
omarchy plugin enable io.github.tilman-schieber.tlog --after omarchy.clock
```

The shell does not notice edits inside a symlinked plugin, so after changing
these files run `omarchy restart shell`.

## A key for capture

The panel answers to IPC, so a Hyprland binding can open it straight into the
capture field from anywhere:

```sh
omarchy-shell io.github.tilman-schieber.tlog capture   # also: toggle, journal, refresh
```

## Tests

`node omarchy/tests/model_test.js` (run by `just test`) covers the grouping,
the keyboard walk and how a capture becomes a tlog command.
