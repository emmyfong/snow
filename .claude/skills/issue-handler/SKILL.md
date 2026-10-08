---
name: issue-handler
description: Handle one epic end to end in Snow, with all its sub-issues, in one worktree, one branch, and one PR that closes the epic and every sub-issue. Gets the user's go-ahead on a plan, builds sub-issues in blocker order with frequent Conventional Commits, opens the PR, and gets CI green. Never merges. Use when the user names an epic or issue to work on, or after next-issue hands one over.
---

# Handle one epic

An epic is the unit of work: one worktree, one branch, one PR. Its sub-issues
are the steps. The PR closes the epic and every sub-issue. The user approves
the plan before you build and merges the PR after you finish.

An issue with no sub-issues and no parent epic (for example, a lone bug) is
handled the same way, as an epic of one.

## 1. Read

```sh
R=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
gh issue view <epic> -R "$R" --json number,title,body,labels
gh api "repos/$R/issues/<epic>/sub_issues" --jq '.[] | "\(.number) \(.state) \(.title)"'
```

For each open sub-issue, read its body and its open blockers:

```sh
gh issue view <n> -R "$R" --json number,title,body,labels
gh api "repos/$R/issues/<n>/dependencies/blocked_by" --jq '.[] | select(.state=="open") | .number'
```

- A blocker inside this epic sets the build order.
- A blocker outside this epic stops that sub-issue. Tell the user which issue
  blocks it. Ask whether to wait, or to leave that sub-issue out of this PR.
- Read `.context/CONTEXT.md` in the main checkout, the ADRs in
  `docs/decisions/` on the topic, and the code the epic touches.
- If an acceptance criterion is ambiguous, ask now, not after you build.

## 2. Set up the worktree

Branch type from the epic's labels: `bug` gives `fix`, `docs` gives `docs`,
`tests` gives `test`, `optimization` gives `perf`, anything else gives `feat`.
The slug is three to five words of the epic title in kebab case.

Shell variables do not survive between Bash calls. Work out the literal
values first, then use them in every later command:

- `<root>`: the output of `git rev-parse --show-toplevel` in the main checkout.
- `<branch>`: `<type>/<epic>-<slug>`, for example `feat/12-shell-in-pane`.
- `<wt>`: `<root>/.worktrees/<branch with / replaced by ->`, for example
  `<root>/.worktrees/feat-12-shell-in-pane`.

```sh
git -C <root> fetch origin
git -C <root> worktree add --no-track -b <branch> <wt> origin/main
ln -s <root>/docs <wt>/docs
```

`--no-track` keeps the branch from tracking `origin/main`, so a bare
`git push` can never target main.

Do all further work inside `<wt>`. Create its `.context/CONTEXT.md` with the
`context` skill. Record `<root>`, `<branch>`, `<wt>`, and the sub-issue order
in it, so a later session has the literal values.

## 3. Plan gate

Show the user a short plan in chat:

- The sub-issues in build order, each with its approach in one or two lines.
- Files to create and modify, with paths.
- Tests: names and what each proves.
- Sub-issues left out, and why.
- Risks or open questions.

Stop and wait for the go-ahead. Revise until the user approves.

## 4. Build

Work one sub-issue at a time, in the approved order.

- Follow `AGENTS.md`: Code quality, Comments, Tests.
- Commit at each working step. Conventional Commit, scope is the package.
  Reference the sub-issue in the body: `Part of #<n>`. No trailers that name
  an agent (see `AGENTS.md`, Authorship).
- Check locally, only the touched packages:
  ```sh
  go build ./...
  go vet ./internal/<pkg>/...
  go test ./internal/<pkg>/...
  ```
- Update `CONTEXT.md` when a sub-issue is done.
- A failure you cannot explain: use superpowers:systematic-debugging.
- Scope grows: stop and ask the user. Put new work in a new sub-issue or a new
  epic, not silently in this PR.

## 5. Open the PR

Before the first push, rebase on main:
`git -C <wt> fetch origin && git -C <wt> rebase origin/main`. After the first
push, never rebase. Merge `origin/main` into the branch instead, because force
push is blocked.

```sh
git -C <wt> push -u origin <branch>
```

Write the body to `<wt>/.context/pr-<epic>.md` from
`.github/pull_request_template.md`. Put one `Closes #<n>` line for the epic and
one for each sub-issue this PR completes. GitHub needs a keyword per issue.
Fill the summary and how it was tested. End the body with the plain line
`Co-authored by Claude Code.` Then:

```sh
gh pr create --title '<type>(<scope>): <summary>' --body-file <wt>/.context/pr-<epic>.md
```

The title is a Conventional Commit. It becomes the squash commit on `main`.
Pass the title in single quotes and the body only through `--body-file`.

If a sub-issue was left out, do not close the epic. Leave out its `Closes`
line and tell the user which sub-issues remain open.

## 6. Get CI green

```sh
gh pr checks <pr> --watch
```

On red, read the failure, fix it, commit, push, and watch again:

```sh
gh run view <run-id> --log-failed | tail -100
```

## 7. Report and stop

Tell the user: the PR link, the CI state, the issues it closes, what changed,
and anything they must decide. Update `CONTEXT.md` with a Handoff. Do not
merge. `gh pr merge` is blocked by a hook.

## 8. After the user merges

Confirm the merge first. A squash merge leaves the branch unmerged in git's
view, so `-D` is required and safe only after this check.

```sh
gh pr view <pr> --json state --jq .state   # must print MERGED
```

Check that the epic and its sub-issues closed. Run the graduation step of the
`context` skill. Then:

```sh
git -C <root> worktree remove <wt>
git -C <root> branch -D <branch>
```

## Spikes

A sub-issue or epic whose acceptance is a written finding answers a question.
Put the finding in the PR body and, if it settles a decision, in an ADR with
the `decision-record` skill. Mark spike code as throwaway in the PR. Keep it
only if the user says so. The PR title type is `docs` for the finding.
