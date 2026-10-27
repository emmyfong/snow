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
  listpick/      the list popup (rows, keys, click mapping) that the mode and resume pickers share
  modepick/      the approval mode picker (`/mode`)
  resumepick/    the past-conversation picker (`/resume`; `Ctrl+a` toggles every folder)
  modelpick/     the model and effort picker (`/model`)
  authpick/      the provider key picker (`/login`)
  helppanel/     the help panel that home and chat share; each screen passes its commands and keys
  command/       the command registry (names, arguments, where each runs, fuzzy match); the home page and the session command popup bind their own behavior
  agents/        the agents board (Ctrl+b w): every agent of the session from api.AgentList over the whole window area, with kind, state, how long, folder, and a chat's model and cost; Enter focuses the agent's pane
  sessionlist/   session rows (name, state, last used, a "needs you" alert) for the home page, and the session list popup (Ctrl+b s)
  confirm/       the confirmation popup (Close/Cancel buttons; y, n, Esc); keys and mouse share it
  menu/          the right-click menu (`menu.PaneItems` reads labels and shortcuts from `keys.Table`)
  whichkey/      the prefix hint popup (`whichkey.View`, fed by `keys.Table.Entries`)
  mouse/         pure hit-testing (pane, border, tab) and border drags; session turns hits into commands
  keys/          the key binding table: tmux defaults plus config; read by session, whichkey, menu
  copymode/      scrollback, search, selection
  chat/          a chat pane: transcript from ChatSnapshot and ChatEvent, tool cells, the approval box, the question box, the task list, the message being typed, scrolling; its slash commands bind through `chat.Register` and the narrow `chat.Host`, and a command that opens a popup asks the session with an `Ask*` constant
  textdiff/      line diffs for edit cells and the approval box
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

Each entry: name, exact signature, one line of use, and notable options.

| Component | Signature | Use |
|---|---|---|
| PaneView | `PaneView(lines []string, width, height int) string` | Draws a pane's styled rows into exactly width by height cells: cuts long rows, pads short ones, blanks missing ones. No chrome, so no theme. |
| Window | `Window(th theme.Theme, o WindowOptions) string` | Draws a window's panes (`WindowPane`: rect, lines, focused) in exactly `Width` by `Height` cells. Cells between panes become borders with the right junction glyph; borders touching the focused pane use `FocusBorder`; cells outside the panes' area are blank. |
| StatusBar | `StatusBar(th theme.Theme, o StatusOptions) string` | One line of exactly `Width` cells: `[session]` and `Tab`s (` 1 name `, active `*`, zoomed glyph, then a space and the `Agent` mark's glyph when the window holds an agent (`MarkIdle`, `MarkWorking`, `MarkNeedsYou`); the padding is part of the tab's click target) on the left; `Right` or, after a one-cell divider and in the error color, `Message` on the right. The bar is `BarFG`/`BarBG` (the accent in built-in themes); the shown window's tab is the bar inverted, in `TabActive`/`TabActiveBG`; any other tab whose agent needs the user is in `PromptFG`/`PromptBG`, bold. When short, drops `Right` first; a `Message` stays and the tabs are cut instead; a `Message` wider than the bar is cut with `…`. A `Prompt` (a question) replaces the whole bar in `PromptFG`/`PromptBG`, as in tmux. |
| Popup | `Popup(th theme.Theme, o PopupOptions) string` | A framed list exactly `Width` wide: `Title` in the top edge, one row per `Items` entry, `Selected` marked `>` and highlighted; optional `Hints[i]` sits right-aligned and muted at the end of row i (a shortcut), and is dropped when it would touch its item. Nothing below 4 cells. |
| CopyView | `CopyView(th theme.Theme, o CopyViewOptions) string` | Draws a pane's history in copy mode into exactly `Width` by `Height` cells: `Lines` cut or padded as `PaneView` does; `Highlights` (`Row`, `Col`, `Len`, `Kind`) painted over them, where a later kind covers an earlier one (`HighlightMatch` in `MatchFG`/`MatchBG`, `HighlightCurrent` in `CurrentFG`/`CurrentBG`, `HighlightSelected` in `SelectedFG`/`SelectedBG`); and `Indicator` (for example `[12/5000]`) in `IndicatorFG`/`IndicatorBG` over the right end of the first row. |
| Overlay | `Overlay(base, box string, x, y int) string` | Draws `box` over `base` at column `x`, row `y`, cutting what falls outside; `base` keeps its size. Use it to place popups. |
| TabSpans | `TabSpans(th theme.Theme, o StatusOptions) []TabSpan` | Where each tab sits in the bar `StatusBar` draws for the same options (`TabSpan`: window index, first column, width), clipped to what is visible, so a click maps to a window. None while a `Prompt` shows. |
| Size | `Size(box string) (width, height int)` | The widest row and the row count of a rendered box, to center a popup or keep it on screen. |
| Panel | `Panel(th theme.Theme, o PanelOptions) string` | A frame exactly `Width` wide around `Body` rows, `Title` in the top edge. Shares its frame with `Popup`. Nothing below 4 cells. |
| Input | `Input(th theme.Theme, o InputOptions) string` | A one-line text field exactly `Width` wide: `Prompt` in the accent color, then `Value` (its end shows when too long) or a muted `Placeholder`; `Focused` draws a cursor. |
| List | `List(th theme.Theme, o ListOptions) string` | Rows of `ListItem` cells with aligned columns, `Selected` marked with the pointer glyph and highlighted; `Height` shows a window scrolled to the selection. An item's `Alert` (for example "● needs you") sits at the row's right end in `PromptFG`/`PromptBG`, selected or not; the columns are cut to keep it. |
| Dollars | `Dollars(c float64) string` | A cost in US dollars to the cent (`$0.42`); below a cent `<$0.01`. The chat top bar and the agents board share it, beside `ShortCount` for tokens. |
| KeyHints | `KeyHints(th theme.Theme, hints []Hint, width int) string` | One row of `Hint`s (key in accent, description muted); hints that do not fit whole are dropped from the end. |
| Wordmark | `Wordmark(th theme.Theme, width int) string` | "snow" in half-block pixels, `WordmarkHeight` rows, centered in `width`, bold in the terminal's own text color (`WordmarkWidth` columns); nothing when too narrow. |
| Transcript | `Transcript(th theme.Theme, o TranscriptOptions) string` | `TranscriptIndent(width)` is the column its text starts at, for rows drawn beside it. A chat transcript at most `Width` wide: `TranscriptEntry` rows (`Kind`: user, agent, thinking, note, error, queued) with a blank row between them. User, agent, and error entries start with a 6-cell badge (`UserFG`/`UserBG`, `AgentFG`/`AgentBG`, `ErrorFG`/`Error`; a `model` entry, for a reply from another model than asked for, uses the error badge too); wrapped lines line up after it. `Cut` adds "(cut off)". Escape sequences and control characters in the text are dropped. Below 15 cells the badge takes its own row. |
| Activity | `Activity(th theme.Theme, o ActivityOptions) string` | One row of exactly `Width` cells in `ActivityFG`/`ActivityBG`: a spinner frame (`Glyphs.Spinner`, by `Frame`), `Text`, and `Hint` right-aligned; the hint is dropped first when short. |
| TextArea | `TextArea(th theme.Theme, o TextAreaOptions) string` | A multi-line text field, each row exactly `Width` wide: `Prompt` then `Value`, hard-wrapped by cell width, or a muted `Placeholder`. At most `Height` rows, scrolled to `Cursor` (a rune index). `TextAreaCursor(o)` gives the cursor cell and `TextAreaRows(o)` the row count. |
| DiffView | `DiffView(th theme.Theme, o DiffOptions) string` | A line diff (`DiffLine`: `DiffKeep`, `DiffAdd`, `DiffRemove`) as rows at most `Width` wide: `+` rows in `Added`, `-` rows in `Removed`, kept rows muted. Kept rows further than `Context` from a change fold into one `...` row; past `MaxRows` a last row counts what was left out. Long lines are cut. `internal/client/textdiff` computes the lines. |
| ToolCell | `ToolCell(th theme.Theme, o ToolCellOptions) string` | One tool call at most `Width` wide: a header with the `Collapsed` or `Expanded` glyph, `Name`, `Subject`, `+Added -Removed`, and `Summary` (or "running", "waiting for you", "failed ..." by `State`), then the caller's `Details` rows when `Expanded`. A failed header is in `Removed`; a `Selected` header fills the width in the selection colors. |
| ChoiceBox | `ChoiceBox(th theme.Theme, o ChoiceOptions) string` | A question or a failure in a frame exactly `Width` wide: `Title` in the top edge, `Body` rows, then `Choices` as buttons with `Selected` highlighted (-1 for none; one row when they fit, else one row each; no button row without choices). Prompt colors, or `ErrorFG`/`Error` with `Alert`. `Reason` swaps the buttons for a text field and its keys; `ReasonPrompt`, `ReasonPlaceholder`, and `ReasonHints` change what the field says (default: a reason for a no). `ChoiceButtons(o)` gives each button's `ButtonSpan` for clicks. Used for the approval box, the question box, and chat failures. |
| TopBar | `TopBar(th theme.Theme, o TopBarOptions) string` | A chat's top row, exactly `Width` cells: `Title` in bold, a rule in `Border`, then `Gauge` (in `Removed` when `GaugeHot`), `Cost`, `Model` in the accent color, and `Mode`. When short it drops the cost, the gauge, the mode, then the title; the model stays. |

## Theme tokens

`internal/client/theme` holds the tokens. `Default()` is **Frost**, chosen by
the user in #33. `Builtin()` lists the 12 shipped themes, and `ByName` finds
one by name for `[theme] name` in `config.toml`: Frost, Campbell, One Half
Dark, Dark+, Solarized Dark, Dracula, Nord, Gruvbox Dark, Tokyo Night,
Catppuccin Mocha, OpenCode, and Clay (inspired by Claude Code; the name may
not be used).

- Color tokens: `Border`, `FocusBorder`, `BarFG`, `BarBG`, `TabActive`,
  `TabActiveBG`, `Accent`, `Muted`, `Success`, `Error` (a strip background),
  `ErrorFG`, `PromptFG`, `PromptBG`, `Selection` with `SelectionFG` (black
  or white for contrast), and copy mode's
  `IndicatorFG`/`IndicatorBG`, `MatchFG`/`MatchBG`, `CurrentFG`/`CurrentBG`,
  `SelectedFG`/`SelectedBG`, and the chat's `UserFG`/`UserBG`,
  `AgentFG`/`AgentBG`, `Queued`, `ActivityFG`/`ActivityBG`, and the diff's
  `Added` and `Removed` (on the terminal's background; built-in themes use
  `Success` and `Error`). A built-in theme sets the copy mode marker and
  current-match pairs itself; matches reuse the prompt colors and the
  selection reuses the list selection.
- `Glyphs`: border lines, junctions, corners, the zoom marker, the active-tab
  mark, the session brackets, the list pointer, the text caret, the spinner frames, the collapsed and expanded markers of a
  tool call, the `Checked`, `Unchecked`, and `Working` marks of a
  multi-select option and a task, and the agent marks `AgentIdle`,
  `AgentWorking`, `AgentNeedsYou` (○ ◐ ●; `For` swaps in `-` `~` `!` on a
  terminal with no colors).
- `Named(name)` reads `[theme] name` from the config: a built-in name,
  ignoring case, or `default`.
- The terminal may be light or dark. Text on the terminal's own background
  uses the terminal's text color or a mid-tone token; text on a token
  background always sets its own color too.
- Snow keeps the terminal's own background and text colors; tokens color
  only Snow's parts.
- `Resolve()` fills tokens a theme leaves empty from Frost. `For(profile)`
  adapts a theme to the terminal's color depth: text that would fall below a
  3:1 contrast after colors are reduced turns black or white.
  `TestTokensDownsample` checks every theme at true color, 256, and 16 colors.
- A new text-on-background pair must join that test.

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
