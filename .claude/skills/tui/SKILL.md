---
name: tui
description: Build or change Snow's terminal UI in internal/client: views, panes, borders, status bar, home page, pickers, menus, popups, themes, or any layout. Covers the theme-token and pure-component rules, the component catalog, the visual review gate (goldens and Freeze screenshots), and terminal compatibility on Windows Terminal, conhost, and WSL. Do not use for server, PTY, protocol, or storage work with no rendered effect.
---

# Snow TUI

`internal/client/theme` is the only source of colors, borders, glyphs, and
spacing. `internal/client/ui` holds pure render components. Each screen is its
own package that composes them and does only structure: joining, sizing,
selection, layout.

## Structure

```
internal/client/
  app.go         the root model: routes between screens, owns the connection
  screen.go      the Screen interface every screen implements
  theme/         tokens and theme definitions
  ui/            pure, reusable render components (catalog below)
  home/          the home page (one screen)
  session/       the attached view: panes, borders, status bar
  picker/        the profile picker popup
  menu/          the right-click menu
  whichkey/      the prefix hint popup
  copymode/      scrollback, search, selection
```

- A screen or popup is a package that implements `Screen` (or a popup
  interface): `Update`, `View`, and a size setter. The root model only routes.
- Redesigning a screen means changing its package. If a redesign needs edits
  in another screen, the boundary is wrong; fix the interface instead.
- Anything two screens draw the same way belongs in `ui`. Screens never import
  each other.

## Rules

- **Theme tokens only.** A view never contains a literal color, border glyph,
  icon, or spacing value. Read them from the resolved theme (`th.Resolve()`).
  A new visual value is a new token in `theme`, with a default.
- **Components are pure.** An exported component takes a `theme.Theme` and an
  options struct and returns a `string`. It holds no state, handles no
  `tea.Msg`, and imports only the standard library, Lip Gloss, Bubbles,
  `charmbracelet/x/ansi`, and `theme`. State and key handling live in the
  `internal/client` models.
- **Width-safe.** Every component takes a width and never renders wider. Use
  `ansi.StringWidth`, not `len`, to measure; wide characters take two cells.
  Zero values and tiny widths render something sane, never panic.
- **Bubble Tea.** Never touch the model from another goroutine. Server
  updates arrive as messages. Render only what is visible.
- **Mouse and keys are equals.** Every action reachable by key is reachable
  by mouse (click, drag, or the right-click menu), and every menu item shows
  its shortcut.
- **No permanent buttons.** Chrome is pane borders and one status bar.
  Discoverability comes from which-key hints, menus, and the home page.
- **Reuse before you build.** Check the catalog first. Extend a component with
  an option instead of writing a near-copy in a screen.
- **Keep the catalog current.** A new or changed component updates the
  catalog below in the same PR.

## Component catalog

Empty until the home-page epic builds the first components. Each entry:
name, exact signature, one line of use, and notable options.

| Component | Signature | Use |
|---|---|---|

## Theme tokens

Defined by the home-page epic, which also picks the default theme. Expected
roles: text, muted text, accent, selection, success, warning, error,
background, surface, border, focused border, status bar, and the glyph and
spacing sets. Record the final list here.

## Visual review gate

Run for every change with a rendered effect.

1. **Goldens.** Add or update `teatest` golden tests for each changed state at
   `80x24` and `120x40`. Update goldens with `go test ./internal/client/... -update`
   only after you have looked at the new output.
2. **Screenshots.** Render each changed golden to PNG with Freeze
   (`go install github.com/charmbracelet/freeze@latest`) into
   `.context/screens/`. If Freeze is missing, stop and tell the user. Do not
   substitute another renderer or skip the step.
3. **Look.** Open every PNG with the image viewer and check:
   - nothing clips, overlaps, or wraps into borders at either size;
   - focus, selection, and the active pane are obvious;
   - wide characters do not shift borders;
   - key hints and menu shortcuts are readable;
   - empty, loading, and error states look intentional.
4. Record what you checked and what you fixed in the PR body. A passing test
   run is not a visual review.

## Terminal compatibility

Snow runs inside another terminal. Test against these:

| Host | Watch for |
|---|---|
| Windows Terminal | The main target. True color, mouse, Unicode all work. |
| conhost (legacy console) | Weak mouse and color support. Detect it and warn once; never crash. |
| WSL panes via `wsl.exe` | Run under ConPTY; resize and wide characters must still line up. |
| Linux terminals (GNOME, Alacritty, kitty) | True color is common but not universal. |
| Snow inside tmux or SSH | Color depth may drop; mouse may be off. |

- **Color depth.** Let Lip Gloss detect the color profile. Every token must
  stay readable when downsampled to 256 and 16 colors.
- **Unicode width.** Measure with `ansi.StringWidth`. Avoid glyphs whose width
  differs between terminals (some emoji, ambiguous-width symbols) in chrome.
- **Resize.** Recompute layout from the new size on every `tea.WindowSizeMsg`.
  Nothing caches a width.
- **Reset styles.** Every styled run ends reset; Lip Gloss does this, so never
  hand-write escape codes in views.
- **Alternate screen and mouse modes** are owned by the client entry point.
  Leave the terminal clean on exit, panic, and detach.

## Verification

```sh
go test ./internal/client/...
```
