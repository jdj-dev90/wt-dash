# wt-dash

A live terminal dashboard for [Worktrunk](https://worktrunk.dev) (`wt`) worktrees across several repos, built for running multiple AI coding agents (Claude Code etc.) in parallel, one worktree per ticket.

```
 wt-dash                                               🤖 working  💬 waiting  updated 12:23:56
── shop-api ─────────────────────────────────────────────────────────────────────────────
       Branch        Status  HEAD±     main↕   Remote  MR/CI     Age   Message
       main          ?^|     +5K                       # ●       25m   chore: bump deps
▌  🤖  PROJ-123       !       +42 -3    ↑2      ⇡1      !118 ✓    10m   feat: login page
   💬  PROJ-131               +7        ↑5              !121 ●    1h    fix: cache headers
── shop-contracts ───────────────────────────────────────────────────────────────────
       main          ^|                                          1h    chore(release): 5.14.1
───────────────────────────────────────────────────────────────────────────────────────────────
 enter open session · n new worktree · x remove · d diff · o open MR · r refresh · ? help · q quit
```

## What it shows

Per repo, every worktree from `wt list --full --format json`:

- **Agent status** from Worktrunk branch markers: 🤖 working, 💬 waiting. Set by Worktrunk's Claude Code plugin (`wt config plugins claude install`).
- Status symbols, uncommitted diff, ahead/behind the default branch and upstream.
- MR/PR number coloured by CI status (✓ passed, ● running, ✗ failed), review approval.
- Age and subject of the head commit.

Refreshes every 5s (`--interval`). CI data is cached by `wt`, so polling is cheap. A desktop notification (`notify-send`) fires when an agent goes from working to waiting (`--no-notify` to disable).

## Keys

| Key | Action |
|---|---|
| `↑↓` / `jk`, `g` / `G` | Move |
| `enter` | Open the worktree's tmux session (via `tmux-sessionizer`) |
| `n` | New worktree in the selected repo (`wt switch --create`, then open its session) |
| `x` | Remove worktree + branch (`wt remove`, which refuses unmerged branches) |
| `d` | Diff since the branch left the default branch, committed and uncommitted, in git's pager |
| `o` | Open the MR/PR in the browser |
| `r` | Refresh now |
| `?` | All keys |
| `q` | Quit |

**Second monitor:** when the dashboard runs in its own tmux session, `enter`/`n` switch the *most recently active other* tmux client, so sessions open on your main screen instead of replacing the dashboard. This is passed to `tmux-sessionizer` as `TMUX_SESSIONIZER_CLIENT`.

## Repo list

One repo path per line, `#` comments, `~` expands, relative paths resolve from the file's directory. First match wins:

1. `wt-dash <file|dir>` (a directory means `<dir>/.wt-dash`)
2. The nearest `.wt-dash` walking up from the current directory, e.g. one per client directory
3. `$XDG_CONFIG_HOME/wt-dash/repos` (default `~/.config/wt-dash/repos`)

```
# ~/code/work/acme/.wt-dash
shop-api
shop-contracts
```

`wt-dash --list` prints the resolved repo paths, for use in other scripts.

## Install

```sh
go install github.com/jdj-dev90/wt-dash@latest
# or into a directory already on PATH:
GOBIN=~/.local/bin go install github.com/jdj-dev90/wt-dash@latest
```

Requires [`wt`](https://worktrunk.dev) on `PATH`. `enter` and `n` expect a `tmux-sessionizer` script on `PATH` that takes a directory; `notify-send`, `xdg-open` and `delta` (as git's pager) are optional.

## Development

```sh
go test ./...
go run . --interval 2s ~/code/work/acme
```

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss). Colours follow the Kanagawa palette.
