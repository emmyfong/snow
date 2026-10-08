# Body sections and writing checklist

## Epic body

```markdown
## Why
<Who needs this and what they cannot do today. Two to four sentences.>

## Concept map
<ASCII diagram of the parts this epic touches, with each child issue placed
where it changes the system.>

## Children
| Order | Sub-issue | Priority | Blocked by |
|---|---|---|---|
| 1 | #12 Run a shell in a pane | High | none |
| 2 | #13 Resize the PTY with the pane | High | #12 |

## Definition of done
- <Observable outcome a user can check.>

## Related
- <Other epics, ADRs by number and title.>
```

## Sub-issue body

```markdown
## Problem
<What is wrong or missing. File paths and measured numbers where they exist.>

## Scope
- Changes: <what this issue changes>
- Does not change: <what stays out of scope>

## Acceptance
- [ ] <Observable outcome. Name the test or fixture that proves it.>

## Diagram
<ASCII diagram of this change only: the data flow, before and after, or the
failure path. More specific than the epic's concept map.>

## Blocked by / Blocks
- Blocked by: #<n> <why>
- Blocks: #<n>

## Related
- <ADRs, issues, docs>
```

## Example sub-issue (short)

```markdown
## Problem
Snow has no way to show a live shell. `internal/term` does not exist yet.

## Scope
- Changes: add `internal/term` with a `Pane` that runs the user's shell
  through a PTY and renders its screen.
- Does not change: layout, splits, or the server.

## Acceptance
- [ ] `TestPaneEcho` starts the shell, sends `echo hi`, and finds `hi` on the
  emulated screen on Linux, macOS, and Windows.

## Diagram
keys ──► Pane.Write ──► PTY ──► shell
screen ◄── vt emulator ◄── PTY output ◄── shell

## Blocked by / Blocks
- Blocked by: none
- Blocks: #13

## Related
- ADR 0001 Go and Bubble Tea v2
```

## Checklist (run on every body)

- ASD-STE100: short sentences, one idea each, active voice, imperative steps,
  plain words.
- No em-dashes: `grep -n '—' .context/compose/*.md` prints nothing.
- Industry-standard terms or terms from the codebase. Same term for the same
  thing in every body (check the glossary).
- Issue references are `#N`. No `NEW-` placeholders remain before posting
  epics.
- The sub-issue diagram shows this step's change, not the epic's map.
- Titles start with one type tag (`[Epic]`, `[Feature]`, `[Bug]`, ...) that
  matches a type label. No priority in titles; every issue has one priority
  label.
- Acceptance items are observable and name their test.
- No internal planning words: no task ids, waves, or model names.
