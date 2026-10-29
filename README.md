# snow

Snow is a terminal for directing AI coding agents. Today it is a fast,
native replacement for tmux on Windows and Linux: shells in panes and
windows, sessions that keep running when you close the terminal, and
sessions that survive a reboot. It runs natively on Windows, and in WSL and
Linux, as one program.

## Install

### Windows

In PowerShell:

```powershell
irm https://raw.githubusercontent.com/emmyfong/snow/main/install.ps1 | iex
```

The script downloads the latest release for your machine, checks it against
the release's checksums, installs `snow.exe` to
`%LOCALAPPDATA%\Programs\snow`, and adds that folder to your user PATH.
Open a new terminal afterwards. For a given version, set
`$env:SNOW_VERSION = "v0.1.0"` before running it.

Or download `snow_windows_amd64.zip` from the
[latest release](https://github.com/emmyfong/snow/releases/latest), unzip
it, and put `snow.exe` in a folder on your `PATH`. Snow runs in Windows
Terminal, and in WSL profiles from Windows.

### Linux and WSL

```sh
curl -fsSL https://raw.githubusercontent.com/emmyfong/snow/main/install.sh | sh
```

The script downloads the latest release for your machine, checks it against
the release's checksums, and installs `snow` to `~/.local/bin`. For a given
version or another folder, set the variables for `sh`, after the pipe:
`curl -fsSL .../install.sh | SNOW_VERSION=v0.1.0 SNOW_INSTALL_DIR=~/bin sh`. On Debian and Ubuntu you can also install the `.deb` from the
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
snow auth      save or remove provider API keys
snow claude    add Snow's hooks to Claude Code's settings
snow version   print the version
snow help      show the commands
```

Inside a session, `Ctrl+b` then `d` goes back to the home page; the session
keeps running. Press `Ctrl+b` and wait half a second to see every key.

### Claude Code panes

Snow runs Claude Code, Anthropic's coding agent, in a pane. Install it from
[claude.com/claude-code](https://claude.com/claude-code) and log in with
`claude` once. Snow uses your own `claude` and your own login: it never
reads, stores, or forwards your Claude Code login or tokens.

`Ctrl+b a` opens a Claude Code pane beside the focused pane, in its folder.
Next to a WSL pane it runs the `claude` inside that distribution. The
profile picker (`Ctrl+b %`, `"`, `c`) lists "Claude Code" when Snow finds
`claude` on your PATH, and "Claude Code (Ubuntu)" for a WSL distribution
that has it and ran when the Snow server started. A distribution started
later joins the picker after the first `Ctrl+b a` in one of its panes,
which always looks again. When Snow finds no `claude`, `Ctrl+b a` says how
to install it. A saved session starts `claude` again in its folder.
Snow finds the Windows side of a WSL pane at `/mnt/<drive>`, WSL's default
mount point.

Run `snow claude setup` once so Snow can show which agent needs you. It
adds one hook, `snow hook`, to `~/.claude/settings.json` (or the file in
`CLAUDE_CONFIG_DIR`), shows the change, and asks first. It keeps every
other setting and hook, and saves a copy as `settings.json.snow.bak`. On
Windows it offers each WSL distribution that has `claude` too.
`snow claude status` says whether the hooks are set, and
`snow claude remove` takes out only Snow's. Claude Code sessions use the
change when they start again. Until the hooks are set, the home page
says so while a Claude Code pane is open. Run `snow claude setup` with
snow under its own name, `snow` or `snow.exe`. A settings file holds one
Snow hook: in a WSL distribution, the Windows `snow.exe` sets it for the
panes of the Windows Snow, and a Linux `snow` there replaces it with its
own when you run setup inside the distribution.

With the hooks set, each window's tab in the status bar shows the state
of its agents after the name: `○` idle, `◐` working, `●` needs you (a
permission question or another answer). A window that needs you is
drawn in the prompt colors. The home page marks a session with "needs
you", and the count when more than one agent waits. Claude Code's hooks do
not say when you answer its question, so a key or a paste you send to a
pane that needs you shows it as working until its next hook. Snow's own chat panes
show the same marks. A terminal with no colors shows `-`, `~`, and `!`.

`Ctrl+b w` opens the agents board: every agent in the session, with its
folder, state, and wait time. `Ctrl+b i` opens the needs-you inbox: every
agent in every session that waits for you, oldest first. Enter jumps to the
selected agent's pane, and you answer there.

### Desktop notifications

Snow shows a desktop notification when an agent starts to need you or
finishes a turn. It skips the notification when the agent's pane is the
focused pane of a Snow window whose terminal has focus, because you already
see the result. Turn the events off in `[notify]`:

```toml
[notify]
needs_you = true      # false: no notification when an agent waits
turn_end = true       # false: no notification when a turn ends
```

### Provider keys

Snow connects to Anthropic (Claude) and OpenAI with your API key.

```text
snow auth login anthropic   check a key and save it
snow auth logout openai     remove a saved key
snow auth status            show which providers have a key
```

Keys go to the OS credential store, or to `~/.snow/credentials` (readable
only by you) where the system has none. `ANTHROPIC_API_KEY` or
`OPENAI_API_KEY`, when set, takes precedence over a saved key.

### Other providers

Snow also connects to any service that speaks the OpenAI Responses API,
such as OmniRoute, OpenRouter, or a local server. Add a
`[provider.<name>]` table to `~/.snow/config.toml`, then restart the
Snow server so it reads the file: stop it and run `snow`. Your sessions
come back as saved sessions. On Linux and macOS:

```sh
pkill -f 'snow server'
```

On Windows, in PowerShell:

```powershell
Get-CimInstance Win32_Process -Filter "Name='snow.exe'" | Where-Object CommandLine -match ' server$' | ForEach-Object { Stop-Process -Id $_.ProcessId }
```

```toml
[provider.omniroute]
base_url = "http://localhost:20128/v1"
no_key = true                  # the service takes no key

[provider.openrouter]
base_url = "https://openrouter.ai/api/v1"
# models = ["qwen/qwen3-coder:free"]   # only for a service with no model list

[provider.ollama]
base_url = "http://localhost:11434/v1"
no_key = true
api = "chat"                   # the service serves Chat Completions only
```

Snow speaks the Responses API to a service by default. Ollama, LM Studio,
llama.cpp, vLLM, and some gateways serve only the Chat Completions API.
For such a service, set `api = "chat"`, and Snow sends each turn to
`<base_url>/chat/completions`. The two APIs are not tried in turn: the
table says which one the service speaks. On the `chat` API Snow sends no
reasoning effort and shows no reasoning, and a service that does not report
token use shows no cost.

Its models show in `/model` as `<name>/<model>`, for example
`openrouter/qwen/qwen3-coder:free`. Save its key with
`snow auth login openrouter` or `/login`, or set `OPENROUTER_API_KEY`.
A key never goes in `config.toml`. A model with no known price shows tokens
instead of a cost. A service with no model list cannot check a key, so
Snow saves it unchecked and says so. Over `http://` the key goes in clear
text, so use `http://` only for a service on your own machine or network.

Use only accounts that the service's vendor allows in other apps.
Anthropic states that apps other than Claude Code and Claude.ai may not
use a Claude Free, Pro, or Max login, and a gateway such as OmniRoute does
not change that: use an API key, or run Claude Code itself in a Snow pane.

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
| `a` | Claude Code pane beside the pane, in its folder |
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
| `s` | Switch to another session from a list |
| `w` | Agents board: every agent in this session |
| `i` | Needs-you inbox: every agent that waits for you |
| `:` | Command list: `new`, `switch`, `rename`, `kill`, `chat` (Snow's own chat), `theme` for this session |
| `d` | Detach: home page; Esc quits Snow |
| `Ctrl+b` | Send `Ctrl+b` to the program |

Typing `exit` in a shell closes its pane, as in tmux. In the last pane it
ends the session and shows the home page; Esc there quits Snow.

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

### Chat

A chat pane talks to Snow's agent in the pane's folder. Open one with `chat`
in `Ctrl+b :`, or bind the `new-chat` action to a key. It sits in the same
windows and layouts as terminal panes. The top bar shows the model, which
is always visible, and the approval mode. It also shows how full the
context is and the estimated cost.

| Key | Does |
|---|---|
| `Enter` | Send the message; during a turn it waits in the queue |
| `Shift+Enter`, `Alt+Enter`, `Ctrl+J` | New line |
| `Esc` | Stop the running turn |
| `PgUp` `PgDn`, arrows, wheel | Scroll; `End` or typing returns to the latest text |
| `Tab`, `Shift+Tab` | Select a tool call; `Enter` or a click shows its details |
| `y` `a` `n` `d` | When a tool asks and the message line is empty: yes, always for this tool, no (with a reason), show the diff or the rest of a long input |
| Arrows, `Tab`, `j` `k`, `1`-`9`, `Space`, `Enter` | When the agent asks a question: move, pick, check (when it takes more than one answer), and answer. "Other" takes a typed answer. After `Esc` the question stays, and only the arrows, `Tab`, `Space`, and `Enter` reach it while the message line is empty; an answer starts a new turn, and a new message moves on |
| `Ctrl+T` | Show or hide the agent's task list above the message line |

#### Chat commands

Type `/` in the message line to see them. Tab completes, Enter runs.

| Command | Does |
|---|---|
| `/model [id]` | Pick a model and its effort, or switch to `provider/model` |
| `/login` | Save a provider key |
| `/logout` | Remove a saved provider key |
| `/new` | Start a new conversation in this pane, in the same folder |
| `/resume` | Open a past conversation; `Ctrl+a` lists every folder |
| `/rename [title]` | Rename the conversation |
| `/mode [ask\|auto]` | Set when tools wait for you; with no word, pick from a list |
| `/effort [level]` | Set the effort from the next turn; `default` lets the provider choose |
| `/help` | Show the commands and keys |

`/new` and `/resume` wait for a running turn to end: press `Esc` first. A
conversation that another pane shows is not opened twice; `/resume` moves
focus to that pane. A model refuses an effort level it does not take, and
the message lists the levels it does.

The approval box lists the whole input of the call. A message holds at most
16 KiB, and at most four messages wait while a turn runs.

### Mouse

Click a pane to focus it, or a window tab to show it. Drag a border to
resize. Right-click for a menu of common actions, each with its key. A program that
uses the mouse, such as `htop` or `vim`, gets the clicks; hold `Ctrl` to
open the menu over it.

## Configuration

Snow reads `~/.snow/config.toml` (`%USERPROFILE%\.snow\config.toml` on
Windows). Every setting is optional. This file shows each one with its
default; the lines starting with `#` are examples to copy:

```toml
[keys]
prefix = "C-b"        # the prefix key, in tmux notation: C- Ctrl, M- Alt, S- Shift

[keys.bind]           # your changes to the keys after the prefix; none by default
# "|" = "split-right" # key = action
# "%" = "none"        # "none" removes a key

[scrollback]
lines = 2000          # lines each pane keeps above its screen

[profile]
default = ""          # the shell new panes start with; empty means the first found
# default = "Ubuntu"

[theme]
name = "default"      # or Frost, Campbell, One Half Dark, Dark+, Solarized Dark,
                      # Dracula, Nord, Gruvbox Dark, Tokyo Night,
                      # Catppuccin Mocha, OpenCode, Clay

[sessions]
save = true           # false saves nothing
lines = 2000          # output lines saved per pane, 0 to 2000

[chat]
model = ""            # "provider/model"; empty: the default of the first
                      # provider with a saved key
effort = ""           # for example "high"; empty: the provider's choice
usage = true          # false hides the context gauge and the cost

[notify]
needs_you = true      # false: no notification when an agent waits
turn_end = true       # false: no notification when a turn ends

# [provider.omniroute]                  # a service that speaks the OpenAI API;
# base_url = "http://localhost:20128/v1"  # see "Other providers"
# no_key = true
# models = []
# api = "responses"                     # or "chat", for a Chat Completions service
```

Actions for `[keys.bind]`: `split-right`, `split-down`, `new-window`,
`new-claude`, `new-chat` (no key by default),
`next-window`, `prev-window`, `select-window-0` to `select-window-9`,
`rename-window`, `close-pane`, `close-window`, `zoom`, `even-out`,
`focus-left`/`right`/`up`/`down`, `resize-left`/`right`/`up`/`down` (by 1),
`resize-left-5` and the others (by 5), `copy-mode`, `sessions`,
`choose-agent`, `inbox`, `command-prompt`, `detach`, `send-prefix`.

A mistake in the file stops Snow with the line and the setting at fault.
`/theme` on the home page, or `theme` in `Ctrl+b :`, changes `[theme] name`
for you, and every open Snow window follows within two seconds.

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
