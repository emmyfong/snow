# snow

Snow is a terminal for directing AI coding agents. Today it is a fast,
native replacement for tmux on Windows and Linux: shells in panes and
windows, sessions that keep running when you close the terminal, and
sessions that survive a reboot. It runs natively on Windows, and in WSL and
Linux, as one program.

## Install

### Windows

With [Scoop](https://scoop.sh):

```powershell
scoop bucket add snow https://github.com/emmyfong/snow
scoop install snow
```

Or download `snow_windows_amd64.zip` from the
[latest release](https://github.com/emmyfong/snow/releases/latest), unzip
it, and put `snow.exe` in a folder on your `PATH`. Snow runs in Windows
Terminal, and in WSL profiles from Windows.

### Linux and WSL

```sh
curl -fsSL https://raw.githubusercontent.com/emmyfong/snow/main/install.sh | sh
```

The script downloads the latest release for your machine, checks it against
the release's checksums, and installs `snow` to `~/.local/bin`. Set
`SNOW_VERSION=v0.1.0` for a given version, or `SNOW_INSTALL_DIR` for another
folder. On Debian and Ubuntu you can also install the `.deb` from the
release: `sudo apt install ./snow_amd64.deb`.

### From source

With Go installed: `go install github.com/emmyfong/snow/cmd/snow@latest`.

## First run

Run `snow`. It starts a background server and opens the home page, which
lists your sessions. Type a name and press Enter to create a session, or
Enter alone for a numbered one. Pick a running or saved session with the
arrow keys and Enter.

```text
snow           open the home page
snow <name>    attach to session <name>, or create it
snow version   print the version
snow help      show the commands
```

Inside a session, `Ctrl+b` then `d` goes back to the home page; the session
keeps running. Press `Ctrl+b` and wait half a second to see every key.

### Home page commands

Type `/` on the home page to see them. Tab completes, Enter runs.

| Command | Does |
|---|---|
| `/new [name]` | Start or open a session |
| `/sessions` | Show the sessions |
| `/rename [session] <name>` | Rename the selected session, or the one named |
| `/kill [session]` | End a session and its programs (asks first) |
| `/theme [name]` | Pick the colors; the list previews each theme |
| `/help` | Show commands and keys |
| `/quit` | Leave Snow; sessions keep running |

## Keys

Every key in a session starts with the prefix, `Ctrl+b`, as in tmux.

| Key after `Ctrl+b` | Does |
|---|---|
| `%` | Split right |
| `"` | Split down |
| `c` | New window |
| `n`, `p` | Next and previous window |
| `0` to `9` | Go to window 0 to 9 |
| `,` | Rename the window |
| `x` | Close the pane (asks first) |
| `&` | Close the window (asks first) |
| `z` | Zoom the pane, or unzoom |
| `E` | Even out the panes |
| Arrows | Focus the pane in that direction |
| `Ctrl` + arrows | Resize the pane by 1 cell |
| `Alt` + arrows | Resize the pane by 5 cells |
| `[` | Copy mode |
| `s` | Home page to switch sessions; Esc comes back |
| `d` | Detach: home page; Esc quits Snow |
| `Ctrl+b` | Send `Ctrl+b` to the program |

Typing `exit` in a shell closes its pane, as in tmux.

### Copy mode

`Ctrl+b [` reads a pane's history. The mouse wheel over a pane, or dragging
across it, opens copy mode too.

| Key | Does |
|---|---|
| Arrows, `h` `j` `k` `l` | Move |
| `PgUp` `PgDn`, `Ctrl+b` `Ctrl+f` | Page up and down |
| `Ctrl+u` `Ctrl+d` | Half a page up and down |
| `0` `$`, `Home` `End` | Start and end of the line |
| `g` `G` | Top and bottom of the history |
| `?` `/` | Search older and newer lines; `n` `N` repeat |
| `v` | Start a selection |
| `y`, `Enter` | Copy the selection and leave |
| `q`, `Esc` | Leave |

### Mouse

Click a pane to focus it, or a window tab to show it. Drag a border to
resize. Right-click for a menu of common actions, each with its key. A program that
uses the mouse, such as `htop` or `vim`, gets the clicks; hold `Ctrl` to
open the menu over it.

## Configuration

Snow reads `~/.snow/config.toml` (`%USERPROFILE%\.snow\config.toml` on
Windows). Every setting is optional; these are the defaults:

```toml
[keys]
prefix = "C-b"        # the prefix key, in tmux notation: C- Ctrl, M- Alt, S- Shift

[keys.bind]           # your changes to the keys after the prefix
"|" = "split-right"   # key = action; "none" removes a key
"-" = "split-down"

[scrollback]
lines = 2000          # lines each pane keeps above its screen

[profile]
default = "Ubuntu"    # the shell new panes start with; empty means the first found

[theme]
name = "default"      # or Frost, Campbell, One Half Dark, Dark+, Solarized Dark,
                      # Dracula, Nord, Gruvbox Dark, Tokyo Night,
                      # Catppuccin Mocha, OpenCode, Clay

[sessions]
save = true           # false saves nothing
lines = 2000          # output lines saved per pane, 0 to 2000
```

Actions for `[keys.bind]`: `split-right`, `split-down`, `new-window`,
`next-window`, `prev-window`, `select-window-0` to `select-window-9`,
`rename-window`, `close-pane`, `close-window`, `zoom`, `even-out`,
`focus-left`/`right`/`up`/`down`, `resize-left`/`right`/`up`/`down` (by 1),
`resize-left-5` and the others (by 5), `copy-mode`, `sessions`, `detach`,
`send-prefix`.

A mistake in the file stops Snow with the line and the setting at fault.
`/theme` on the home page changes `[theme] name` for you, and every open
Snow window follows within two seconds.

## Saved sessions

Snow saves each session to `~/.snow/sessions/` so it survives a reboot or a
server restart: its windows, layout, each pane's shell and folder, and the
last 2,000 lines of each pane's output. Opening a saved session on the home
page starts fresh shells in the same places, below the saved output. On
Windows, the shell clears the screen as it starts, so the saved output is
one page up: scroll with the mouse wheel or use copy mode.

Saved output can include anything a program printed, such as tokens or
passwords. The folder is readable only by you, and a session's file stays
until the session is killed. To save less, set `[sessions] lines = 0` (only
the layout and folders) or `save = false`.

A WSL pane that ends because WSL stopped (`wsl --shutdown`) stays on screen;
press `r` in it to start it again in the same folder.

A saved file that cannot be read moves to `~/.snow/sessions/broken/`, and
the home page says so.

## Updating

After an update, the server from the old version may still be running. A
newer `snow` notices and asks to restart it; the server saves its sessions
first, and they come back as saved sessions. Programs running in them stop.

## License

MIT
