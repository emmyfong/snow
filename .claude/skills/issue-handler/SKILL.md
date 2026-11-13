---
name: issue-handler
description: Handle one epic end to end in Snow, with all its sub-issues, in one worktree, one branch, and one PR that closes the epic and every sub-issue. Gets the user's go-ahead on a plan, builds sub-issues in blocker order with milestone Conventional Commits, runs a review wave on the branch, and opens the PR. Leaves CI and the merge to the user. Use when the user names an epic or issue to work on, or after next-issue hands one over.
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
- Check whether the issue is active, parked, partly shipped, or superseded.
  Resolve conflicting scope, acceptance criteria, or ADRs before dependent work.
- If the user problem, scope, or design is unclear, use `plan-with-user` before
  implementation. Ask about consequential ambiguity; retain previous answers.

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

Create the branch on GitHub, linked to the epic, so the epic shows it under
Development and other agents see the work has started. Then check it out in
a worktree:

```sh
gh issue develop <epic> -R "$R" --name <branch> --base main
git -C <root> fetch origin <branch>
git -C <root> worktree add --no-track -b <branch> <wt> origin/<branch>
ln -s <root>/docs <wt>/docs
```

`--no-track` keeps the new local branch without an upstream until the first
`git push -u`, so a bare `git push` never targets another branch.

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

Obtain approval for the concrete plan before building. If the user has already
approved this scope and approach, use that authorization. Do not ask again for
routine implementation choices. Resolve remaining product choices before work
that depends on them.

## 4. Build

Work one sub-issue at a time, in the approved order.

- Follow `AGENTS.md`: Code quality, Comments, Tests.
- Commit at milestones: usually one commit per finished sub-issue, more only
  for a tested step worth returning to. No work-in-progress or fix-up commits.
  Conventional Commit, scope is the package. Reference the sub-issue in the
  body: `Part of #<n>`. No trailers that name
  an agent (see `AGENTS.md`, Authorship).
- Check locally, only the touched packages:
  ```sh
  go build ./...
  go vet ./internal/<pkg>/...
  go test ./internal/<pkg>/...
  ```
  For documentation-only work, use the documentation checks in
  `review-wave/references/local-gate.md`; Go checks are not applicable.
- Update `CONTEXT.md` when a sub-issue is done.
- Investigate unexplained failures systematically. Use a debugging skill if one
  is available; do not require an absent plugin. Repeated attempts should add evidence.
- Scope grows: pause the affected work and ask about the consequential change.
  Continue independent authorized work. Put new scope in an agreed sub-issue or
  epic, not silently in this PR. For a skill improvement, use `compose-skill`:
  propose it, obtain approval, and use a dedicated issue, worktree, and PR.

## 5. Review the branch

Before the PR, run the `review-wave` skill on the branch. It resolves reviewer
settings from prior answers or asks for missing choices, then writes a curated report.

- Show the user the report and let them choose which findings to fix.
- Fix the chosen findings. Behavior fixes get a test that fails first.
- Commit the fixes as milestone commits.
- Findings the user defers become new issues, or lines in the PR body.

## 6. Open the PR

Run the local gate (`review-wave/references/local-gate.md`) on the final
branch. Open the PR only when every check passes, and list the results in
the PR body.

Before the first push, rebase on main:
`git -C <wt> fetch origin && git -C <wt> rebase origin/main`. After the first
push, never rebase. Merge `origin/main` into the branch instead, because force
push is blocked.

```sh
git -C <wt> push -u origin <branch>
```

Offer to post the review report as the PR's first comment.

Write the body to `<wt>/.context/pr-<epic>.md` from
`.github/pull_request_template.md`. Put one `Closes #<n>` line for the epic and
one for each sub-issue this PR completes. GitHub needs a keyword per issue.
Fill the summary and how it was tested. A dedicated skill PR includes its
changelog entries and validation. Follow AGENTS.md authorship rules. Then:

```sh
gh pr create --title '<type>(<scope>): <summary>' --body-file <wt>/.context/pr-<epic>.md
```

The title is a Conventional Commit. It becomes the squash commit on `main`.
Pass the title in single quotes and the body only through `--body-file`.

If a sub-issue was left out, do not close the epic. Leave out its `Closes`
line and tell the user which sub-issues remain open.

GitHub closes a linked epic when its PR merges, even without a `Closes`
line, because `gh issue develop` linked the branch to it. So when a
sub-issue was left out, tell the user in the step 7 report that the epic
will close on merge. In step 8, after you confirm the merge, reopen the
epic with a comment that names the open sub-issues.

## 7. Report and stop

Tell the user: the PR link, the issues it closes, what changed, and anything
they must decide. Do not watch CI: it runs on its own, and the user handles
failures and merges. Update `CONTEXT.md` with a Handoff. Do not merge.
`gh pr merge` is blocked by a hook.

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

## Improve this skill

When a step here was wrong, missing, or unclear, propose a correction through
`compose-skill`. Obtain user approval before editing unless that change is already
authorized. Use a dedicated skill issue, worktree, and PR. Continue independent
task work while approval is pending.
