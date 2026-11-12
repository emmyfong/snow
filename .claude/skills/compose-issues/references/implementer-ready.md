# Implementer-ready sub-issues

Another agent (Claude Code, Codex, or OpenCode) builds each sub-issue alone,
with only `AGENTS.md`, the skills, the code, and the issue. It cannot ask the
planner what was meant. A sub-issue is ready when that agent can start, finish,
and prove the work without a question to the user.

Add these sections to the sub-issue body from `body-sections.md`, after
`## Scope`. Keep each one short and exact.

````markdown
## Starting points
- `internal/<pkg>/<file>.go`: `<Func or Type>` <what it does now, in one line>
- <Every file the change reads or edits, found by reading the code, not guessed.>

## Design
- <The interface, type, message, or config key to add, as a Go signature or a
  TOML snippet. Name the package it lives in.>
- <The data flow in one or two lines, or point to the diagram.>
- <Which goroutine owns new state (ADR 0007). Which runtime this supports.>

## Steps
1. <Imperative step that leaves the build green.>
2. <...>

## Verify
```sh
go build ./...
go vet ./internal/<pkg>/...
go test ./internal/<pkg>/... -run '<TestName>'
```
- <For a rendered change: the golden names at 80x24 and 120x40, and the tui
  skill's screenshot gate.>
- <For Windows behavior: `GOOS=windows go test -c` and run the binary.>

## Traps
- <Known pitfalls in this code: locks around writes, the owner goroutine,
  ConPTY, WSL paths, flaky tests to skip (with issue number).>

## Decisions this follows
- <ADR numbers and titles, DESIGN.md sections, and user decisions with dates.>

## Open questions
- <None, or each question with the planner's recommendation. A sub-issue with
  an open question that changes behavior is not ready: mark it blocked.>
````

## Rules

- **Read the code first.** Every path, function, type, and test name must exist
  at the planning commit, or be marked "new". Record that commit in the epic.
- **Name the tests.** Each acceptance item names the test function that proves
  it and the package it lives in. Behavior fixes name the failing test first.
- **One owner per change.** Two sub-issues never edit the same function in
  parallel. If they must, add a blocked-by link.
- **Say what not to do.** List the tempting wider change in `Does not change`.
- **No hidden decisions.** If the design picks between options, write the
  choice and its reason. If the user has not chosen, it is an open question.
- **Size.** One sub-issue is one to three commits and about one day of agent
  work. Split anything larger.

## Readiness check (the verifier runs this)

- [ ] Every cited path and symbol exists at the planning commit, or says "new".
- [ ] Every acceptance item names a test, and the Verify block runs it.
- [ ] The design names the package, the types, and the owning goroutine.
- [ ] No open question changes behavior, or the sub-issue is marked blocked.
- [ ] `Does not change` lists the nearest out-of-scope work.
- [ ] Blocked-by links match the epic DAG.
- [ ] No conflict with an accepted ADR, `DESIGN.md`, or `AGENTS.md`.
- [ ] Body checklist in `body-sections.md` passes.
