# Snow

Snow is a cross-platform terminal app for directing AI coding agents. It is one
Go binary that runs a local server and a Bubble Tea TUI client. It never hosts
models; users connect the agents and providers they already have.

Design decisions live in `docs/decisions/` (local, not committed). Read the
relevant ADR before you change a decided topic. Do not reopen a decision
without the user.

## Code layout

Go standard layout. No `src/`.

```
cmd/snow/            the binary; thin wiring only
internal/<pillar>/   product code, one package per concern:
                     term, layout, shell, server, client, store,
                     later: backend, skills, context, gate, editor,
                     hook, worktree
pkg/api/             the only public package: API types for other clients
```

- One folder is one package. Tests sit beside the code (`x_test.go`).
- Platform code uses filename suffixes (`_windows.go`, `_unix.go`), not runtime
  checks. Go treats `_windows.go` and `_linux.go` as build constraints, but not
  `_unix.go`: start every `_unix.go` file with `//go:build !windows`.
- Grow by adding packages or sub-packages. Do not add top-level folders.

Import direction. depguard fails CI on a wrong import.

```
cmd/snow ──► client ──► pkg/api ◄── server ──► backend, gate, skills, store
                                       │
                                       └──► term, layout   (leaf packages)
```

## Code quality

- `gofmt`, `go vet`, and `golangci-lint` must be clean. CI enforces them.
- Accept interfaces, return structs. Wrap `charmbracelet/x` packages behind
  Snow's own interfaces in `internal/term`; they are experimental.
- Wrap errors with `%w`. Never discard an error. Return typed errors at package
  boundaries so callers can act on the kind of failure.
- Bubble Tea: never touch the model from another goroutine. Deliver PTY output
  and server events as messages.
- No cgo. Never assume bash; shells differ per OS.

## Modularity

Snow must be easy to iterate on: a feature can be redesigned by replacing its
folder, without edits across the codebase.

- **Reuse first.** Before you write a helper or a UI component, search for one
  (`grep`, the catalog in the `tui` skill). Extend it instead of copying it.
- **Extract on the second copy.** When the same logic appears a second time,
  move it into a shared package and call it from both places. Do not build an
  abstraction for a single caller.
- **One concern per file.** Name each file after what it holds (`border.go`,
  `picker.go`). Split a file when it mixes responsibilities or passes about
  300 lines.
- **Folders mean something.** Each feature lives in its own package, for
  example `internal/client/home/`. Changing a feature should touch its folder,
  not the rest of the app.
- **Plug in through small interfaces.** Features connect through a narrow
  interface (for example, a client `Screen`), so one can be replaced without
  editing the others.

## Comments

- Doc comments on every exported identifier.
- Inside functions, comment only the non-obvious why: a workaround, an
  invariant, a platform quirk. Never narrate what the code does.
- No commented-out code. No `TODO` without an issue reference: `TODO(#12)`.

## Tests

- A change in behavior ships with tests for that behavior in the same PR.
- A bug fix starts with a failing regression test.
- Prefer table-driven tests. Use `teatest` golden files for UI and real PTYs
  for terminal code.
- Do not mock Snow's own packages. Fakes are allowed at the `Backend` and
  `Gate` boundaries.
- Locally, run only: `go build ./...`, then `go vet` and `go test` on the
  packages you touched. Do not run the full suite. CI runs everything on
  Linux, macOS, and Windows and gates the merge.

## Issues and worktrees

- Work is tracked only in GitHub Issues.
- An epic (label `epic`) is one unit of work: one worktree, one branch, one PR.
  Its sub-issues are the steps. A standalone issue counts as an epic of one.
- Take on an epic with the `issue-handler` skill, in its own worktree under
  `.worktrees/`. Never build an epic in the main checkout.
- Sub-issues may be blocked by other issues. A blocker inside the epic sets
  the build order. A blocker outside the epic stops that sub-issue; ask the
  user.
- The epic's PR closes the epic and every sub-issue it completes, with one
  `Closes #N` line per issue.
- Labels: one priority (`Priority: Critical`, `High`, `Medium`, `Low`) and one
  or more types (`feature`, `enhancement`, `bug`, `optimization`, `tests`,
  `docs`).
- Titles start with one type tag: `[Epic]` for epics, otherwise the main type
  label: `[Feature]`, `[Enhancement]`, `[Bug]`, `[Optimization]`, `[Tests]`,
  `[Docs]`. Example: `[Feature] Detect shell profiles`. Priority stays in
  labels only.

## Git

- Conventional Commits: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`,
  `ci`, `perf`. The scope is the package: `feat(term): add resize`.
- Commit at milestones worth saving: a finished sub-issue, or a tested step
  you would want to return to. No work-in-progress or fix-up commits; amend
  local commits before the first push instead.
- Branch: `<type>/<epic#>-<slug>`, for example `feat/12-shell-in-pane`.
- The PR title is a Conventional Commit. It becomes the squash commit on
  `main`.
- Before opening a PR, run the `review-wave` skill on the branch and fix the
  findings the user chooses.
- Open the PR and stop. Do not watch CI. The user handles CI failures and
  merges.
- Only the user merges.

## Authorship

- Every commit belongs to the user: author, committer, and push all use the
  user's configured git identity and `gh` login. Never set or change them.
- Commit messages carry no trailers that name an agent: no `Co-Authored-By`,
  no `Generated with`. GitHub links them to accounts and lists the agent as a
  contributor.
- Credit the agent only in a PR description, as the plain last line
  `Co-authored by Claude Code.` Not the trailer form `Name <email>`, because
  the PR body becomes the squash commit message.
- Never force push. Never push to `main`. Never change git config.
- Hooks enforce these rules. If a command is blocked, stop and tell the user.

## Working memory

- At session start, read `.context/CONTEXT.md` if it exists. Read its Handoff
  section first.
- Keep it current with the `context` skill: after milestones, before
  compaction, and at every handoff or model switch.

## Writing on GitHub

Issues, PRs, and review comments use ASD-STE100 style:

- Short sentences. One idea per sentence. Active voice.
- Imperative steps. Plain words. No em-dashes.
- Industry-standard terms or terms already in the codebase. Do not invent terms.
- Refer to code with file paths and issue numbers (`#N`).
