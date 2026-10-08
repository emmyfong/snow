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
                     term, layout, server, client, backend, skills,
                     context, gate, editor, store, hook, worktree
pkg/api/             the only public package: API types for other clients
```

- One folder is one package. Tests sit beside the code (`x_test.go`).
- Platform code uses filename suffixes (`_windows.go`, `_unix.go`), not runtime
  checks.
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
- Keep files small and focused. Add an abstraction only when a second caller
  needs it.

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

## Git

- Conventional Commits: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`,
  `ci`, `perf`. The scope is the package: `feat(term): add resize`.
- Commit often, at each working step.
- One issue, one branch, one PR. Branch: `<type>/<issue#>-<slug>`. Build it in
  a worktree under `.worktrees/`.
- The PR title is a Conventional Commit. It becomes the squash commit on
  `main`. The PR body contains `Closes #N`.
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
