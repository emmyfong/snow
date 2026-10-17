# snow

## Saved sessions

Snow saves each session to `~/.snow/sessions/` so it survives a reboot or
a server restart: its windows, layout, each pane's shell and folder, and
the last 2,000 lines of each pane's output. Opening a saved session on the
home page starts fresh shells in the same places, below the saved output.

Saved output can include anything a program printed, such as tokens or
passwords. The files are readable only by you, but they stay on disk until
the session is killed. To save less, set this in `~/.snow/config.toml`:

```toml
[sessions]
save = true   # false saves nothing
lines = 2000  # 0 keeps only the layout and folders
```

A saved file that cannot be read moves to `~/.snow/sessions/broken/`, and
the home page says so.
