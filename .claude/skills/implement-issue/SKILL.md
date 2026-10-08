---
name: implement-issue
description: Build one GitHub issue end to end in Snow. Creates a worktree and branch, gets the user's go-ahead on a short plan, builds with frequent Conventional Commits, opens a PR, and gets CI green. Never merges. Use when the user names an issue to implement, or after next-issue hands one over.
---

# Implement one issue

One issue, one worktree, one branch, one PR. The user approves the plan
before you build and merges the PR after you finish.

## 1. Read

```sh
gh issue view <n> --json number,title,body,labels
```

- Read the parent epic if the body names one.
- Read `.context/CONTEXT.md` in the main checkout if it mentions this issue.
- Search `docs/decisions/` for ADRs on the topic. Follow them.
- Read the code the issue touches.

If an acceptance criterion is ambiguous, ask the user now, not after you
build.

## 2. Set up the worktree

Branch type from the labels: `bug` gives `fix`, `documentation` gives `docs`,
`ci` gives `ci`, `spike` gives `spike`, anything else gives `feat`. The slug is
three to five words of the title in kebab case.

Shell variables do not survive between Bash calls. Work out the literal
values first, then use them in every later command:

- `<root>`: the output of `git rev-parse --show-toplevel` in the main checkout.
- `<branch>`: `<type>/<n>-<slug>`, for example `feat/12-shell-in-pane`.
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
`context` skill. Record `<root>`, `<branch>`, and `<wt>` in its Files that
matter section, so a later session has the literal values.

## 3. Plan gate

Show the user a short plan in chat:

- Approach: two to five sentences.
- Files: create and modify, with paths.
- Tests: the test names and what each proves.
- Risks or open questions.

Stop and wait for the go-ahead. Revise until the user approves.

## 4. Build

- Follow `AGENTS.md`: Code quality, Comments, Tests.
- Commit at each working step. Conventional Commit, scope is the package. End
  each message with the `Co-Authored-By` trailer for your model.
- Update `CONTEXT.md` at each milestone.
- Check locally, only the touched packages:
  ```sh
  go build ./...
  go vet ./internal/<pkg>/...
  go test ./internal/<pkg>/...
  ```
- A failure you cannot explain: use superpowers:systematic-debugging.
- Scope grows: stop and ask the user. Put extra work in a new issue, not in
  this PR.

## 5. Open the PR

Before the first push, rebase on main: `git -C <wt> fetch origin && git -C <wt> rebase origin/main`.
After the first push, never rebase. Merge `origin/main` into the branch
instead, because force push is blocked.

```sh
git -C <wt> push -u origin <branch>
```

Write the body to `<wt>/.context/pr-<n>.md` from `.github/pull_request_template.md`.
Fill `Closes #<n>`, the summary, and how it was tested. End the body with the
`Co-Authored-By` line. Then:

```sh
gh pr create --title '<type>(<scope>): <summary>' --body-file <wt>/.context/pr-<n>.md
```

The title is a Conventional Commit. It becomes the squash commit on `main`.
The title type is not always the branch type: a spike branch uses `docs` for
a written finding, or `chore` for throwaway code.
Pass the title in single quotes and the body only through `--body-file`.

## 6. Get CI green

```sh
gh pr checks <pr> --watch
```

On red, read the failure, fix it, commit, push, and watch again:

```sh
gh run view <run-id> --log-failed | tail -100
```

## 7. Report and stop

Tell the user: the PR link, the CI state, what changed, and anything they
must decide. Update `CONTEXT.md` with a Handoff. Do not merge. `gh pr merge`
is blocked by a hook.

## 8. After the user merges

Confirm the merge first. A squash merge leaves the branch unmerged in git's
view, so `-D` is required and safe only after this check.

```sh
gh pr view <pr> --json state --jq .state   # must print MERGED
```

Run the graduation step of the `context` skill. Then:

```sh
git -C <root> worktree remove <wt>
git -C <root> branch -D <branch>
```

## Spikes

An issue with the label `spike` answers a question. Its output is a written
finding: put it in the PR body and, if it settles a decision, in an ADR with
the `decision-record` skill. Mark spike code as throwaway in the PR. Keep it
only if the user says so.
