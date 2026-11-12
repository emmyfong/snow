# Snow

Snow is a cross-platform terminal app for directing AI coding agents. It is one
Go binary that runs a local server and a Bubble Tea TUI client. It never hosts
models; users connect the agents and providers they already have.

Design decisions live in `docs/decisions/` (local, not committed). Read the
relevant ADR before you change a decided topic. Do not reopen a decision
without the user.

## Planning and collaboration

Clarify goals, scope, and design before implementing ambiguous product work.
Use the user's previous answers and inspect available project context first.
Ask when an unresolved choice changes behavior, scope, cost, persistent data,
permissions, compatibility, or a substantial design commitment. Explain the
tradeoff and offer a recommendation. Handle routine details independently.

State requirements, proposals, assumptions, and unresolved choices separately.
Compare the smallest adequate approach with broader alternatives when it matters.
Reuse existing components and justify new abstractions. Judge code by required
behavior and maintainability, not a line count. Revisit the affected decision if
new evidence invalidates the plan or materially expands scope.

Keep the user informed about consequential findings. Continue independent work
while a question is pending; leave dependent work pending. A request to explore
or plan does not authorize implementation. Honor approval already given without
asking for it again.

## Shared skills

Project skills live in `.claude/skills/`. All agents can read these Markdown
instructions. Automatic discovery and slash commands depend on the agent host;
if discovery is absent, open the applicable `SKILL.md` by path. Keep one shared
copy rather than duplicate skills in agent-specific folders. Read only skills
relevant to the current task and their needed references.

- `plan-with-user`: clarify an idea and develop its scope and design.
- `compose-issues`: turn agreed work into issues or revise existing issues.
- `next-issue`: select ready work from current issues.
- `issue-handler`: implement one agreed issue or epic in a worktree.
- `review-wave`: review a branch or PR before publication.
- `context`: save current working memory and handoffs.
- `decision-record`: record an agreed architecture decision.
- `evaluate-context`: evaluate context policies by task quality and total usage.
- `tui`: design or change Snow's terminal interface.
- `plan-program`: plan many epics with drafting and verifying subagents.
- `agent-orchestration`: coordinate parallel issue implementation and verification teams.
- `manage-subagents`: manage delegated objectives, capacity, verification, and handoffs.
- `compose-skill`: create or fix a skill, test it, and log it for review.

Skills improve themselves. When a skill step fails or the user corrects you
while you follow one, fix the skill with `compose-skill` and log the change in
`.claude/skills/CHANGELOG.md`. The fix may ship in your task's PR, in its own
commit. The user reviews logged changes in batches.

Use tools available in the active environment. A Claude-specific tool name in a
skill is not a requirement to install a plugin or invent an unavailable tool.

## Product and architecture status

Snow's primary direction is its own workspace and interface backed by runtimes
such as Claude Code, Codex, and OpenCode, using supported account connections.
Snow sends the user's task and selected context and supplies shared tools. The
runtime owns its agent loop and model requests. The existing direct-API agent
loop is secondary; no decision has been made to remove it.
Read accepted ADRs and current issue scope before changing either path. Surface
conflicts with older decisions rather than silently treating a proposal as settled.

State which runtime a capability supports. External runtimes own their model
requests and active context. Snow controls its workspace and the tools it exposes.
Prompt instructions alone do not enforce permissions. Show unavailable runtime
telemetry as unavailable, and distinguish API cost estimates from account allowance.

Evaluate context changes by task quality and total usage, including preparation,
coordination, and rework. Preserve the user's original request and agreed decisions.
Do not assume JSON prompts save tokens, rewrite every prompt with another model,
or promise control over an external runtime's caching. Measure before adopting a
context policy. Keep unresolved requirements explicit.

`docs/` and `.context/` are local and ignored by git. Shared rules belong here,
shared workflows in `.claude/skills/`, and execution scope in GitHub Issues.
Read [DESIGN.md](DESIGN.md) for shared product direction, current architecture,
proposed boundaries, and open decisions. Its proposals do not authorize work.

## Code layout

Go standard layout. No `src/`.

```
cmd/snow/            the binary; thin wiring only
internal/<pillar>/   product code, one package per concern:
                     term, layout, shell, transport, server, client,
                     store, config, provider, credential, signin,
                     convstore, agent, tool, claudecode, clauderuntime,
                     gateway, notify,
                     mcp, worktree, git, later: skills, gate, editor
pkg/api/             the only public package: API types for other clients
```

- One folder is one package. Tests sit beside the code (`x_test.go`).
- Platform code uses filename suffixes (`_windows.go`, `_unix.go`), not runtime
  checks. Go treats `_windows.go` and `_linux.go` as build constraints, but not
  `_unix.go`: start every `_unix.go` file with `//go:build !windows`.
- Grow by adding packages or sub-packages. Do not add top-level folders.

Import direction. depguard fails CI on a wrong import.

```
cmd/snow ──► client ──► pkg/api ◄── server ──► provider, credential, gate, skills, store, convstore, notify, private
   │           │                       │
   │           │                       ├──► agent ──► provider, convstore, tool ──► shell
   │           │                       │    (the server runs agents; agents never import server,
   │           │                       │     client, credential, or signin; tools never import
   │           │                       │     agent, server, convstore, provider, or credential)
   │           │                       ├──► clauderuntime ──► shell, private, convstore, pkg/api
   │           │                       │    (Snow's own Claude Code connections: sign-in, status,
   │           │                       │    and the chat runtime, natively or in a WSL distribution)
   │           │                       ├──► worktree, git ──► shell  (worktree: a git worktree
   │           │                       │    per coding agent; git: runs the git command, in
   │           │                       │    the WSL distribution for a WSL folder, for worktree
   │           │                       │    and the git pane; worktree imports only git and
   │           │                       │    shell, git only shell)
   ├──► claudecode ──► pkg/api, transport, git  (snow hook, snow claude: Claude
   │           │                       │    Code's hooks and settings only, never its
   │           │                       │    login; git reads HEAD around an agent's commit)
   ├──► mcp ──► pkg/api, transport     (snow mcp: the pane tools Claude Code
   │           │                       │    calls, through the Snow socket)
   ├──► shell  (snow claude: the WSL distributions that have claude)
   ├──► gateway ──► shell, private, pkg/api  (a configured gateway: starts it,
   │                                          logs it, and stops only a tree
   │                                          Snow started)
   ├──► notify ──► shell  (snow: desktop notifications, hidden on Windows)
   └──► signin ──► provider, credential   (snow auth; later /login on the server)
               │                       │
               │                       │    (provider never imports credential;
               │                       │     keys reach it through KeySource;
               │                       │     convstore never imports provider)
               │                       └──► term, layout, shell   (leaf packages)
               └──────────► config ◄───┘    (leaf; read by client and server)
```

## Code quality

- `gofmt`, `go vet`, and `golangci-lint` must be clean. CI enforces them.
  CI pins the golangci-lint version in `.github/workflows/`. Run that same
  version locally before you push; another version reports other issues.
  After you change a depguard rule, run `golangci-lint cache clean`: a stale
  cache hides the new rule.
- Build that golangci-lint with the Go version in `go.mod`, or it refuses to
  load the config ("lower than the targeted Go version"):
  `GOTOOLCHAIN=go<version from go.mod> go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<pinned version>`.
- Accept interfaces, return structs. Wrap `charmbracelet/x` packages behind
  Snow's own interfaces in `internal/term`; they are experimental.
- Wrap errors with `%w`. Never discard an error. Return typed errors at package
  boundaries so callers can act on the kind of failure.
- Bubble Tea: never touch the model from another goroutine. Deliver PTY output
  and server events as messages.
- No cgo. Never assume bash; shells differ per OS.
- Never hold a lock while you write to a PTY, a pipe, or a socket. A program
  that stops reading blocks the write, and every reader of that lock with it.
- The server's state belongs to one goroutine (ADR 0007). Add closures and
  fields, not locks. Code in that goroutine must never block.
- A provider keeps its key on its vendor's host. Turn off the vendor SDK's
  environment defaults (base URL, extra tokens, organization, profiles),
  refuse redirects to another host (`internal/provider/providerhttp`),
  rebuild SDK errors from the status and message only (the SDK's error
  holds the request and its key header), and turn off SDK retries. Snow
  never retries or switches models by itself.
- On Windows, start background processes with `CREATE_NO_WINDOW`, never
  `DETACHED_PROCESS`. A process with no console makes each console program it
  runs open a visible window.

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
- Do not mock Snow's own packages. Fakes are allowed at the `Provider` and
  `Gate` boundaries (`internal/provider/providertest` has the Provider fake).
- Tests that speak the protocol use `internal/apitest`: its client rebuilds
  the screen from updates, and it gives short socket paths. Extend it; do not
  write another client.
- Real-shell tests wait for the prompt before typing: shells discard input
  that arrives while they start. Match only screen text that cannot wrap;
  widen the pane or shorten the expected text.
- Real-shell tests wait for a `$` prompt, so they fail when run as root,
  whose prompt is `#`. Run them as a normal user. In a container that runs
  as root, run the other tests with `-run` or `-skip` and leave these to CI.
- Timing tests warm up first: Windows antivirus scans each new file on its
  first open (about 1.5 s per 1000 files), so the first pass is not the
  speed you measure. Timing limits skip under `-race`: a `raceDetector`
  constant from a `race_on_test.go` / `race_off_test.go` build-tag pair
  (see `internal/convstore`) tells the test.
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
- The user is the sole named author. Add no agent credit, co-author trailers,
  or generated-by notices to commits or PR descriptions. PR descriptions can
  become squash commit messages. Use the user's existing account for publication.
- Do not rewrite published history or change account metadata to alter attribution.
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
