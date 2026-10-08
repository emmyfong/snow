---
name: next-issue
description: Choose the next GitHub issue to work on in Snow. Reads open issues, priorities, epics, and blocked-by links, drops blocked or in-progress work, and offers the top three for the user to pick. Use when the user asks what to work on next, or before starting implement-issue without a named issue.
---

# Choose the next issue

Find the open issues that are ready to start, rank them, and let the user pick.
This skill reads only. It changes nothing on GitHub.

## 1. Collect

```sh
R=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
gh issue list -R "$R" --state open --limit 200 \
  --json number,title,labels,assignees > .context/issues.json
gh pr list -R "$R" --state open --limit 100 \
  --json number,headRefName,closingIssuesReferences > .context/prs.json
git worktree list --porcelain | grep '^branch ' | sed 's#branch refs/heads/##'
```

For each open epic (label `epic`), list its children in order:

```sh
gh api "repos/$R/issues/<epic>/sub_issues" --jq '.[] | select(.state=="open") | .number'
```

For each candidate issue, list its open blockers:

```sh
gh api "repos/$R/issues/<n>/dependencies/blocked_by" --jq '.[] | select(.state=="open") | .number'
```

## 2. Filter

Drop an issue when any of these is true:

- It is an epic (label `epic`).
- It has an open blocker.
- It has an assignee other than the user.
- An open PR closes it (`closingIssuesReferences`).
- A local branch for it exists in a worktree (branch `<type>/<n>-<slug>`).

## 3. Rank

1. Priority from the title: `[P0]` first, then `[P1]`, `[P2]`, `[P3]`. A title
   with no priority ranks last. Report it as a defect.
2. Then the parent epic's priority.
3. Then the order of the issue among its epic's sub-issues.
4. Then the lower issue number.

Then prefer independence. If work is already in progress in a worktree, move
an issue down one place when it shares a component label (`term`, `layout`,
`server`, ...) with that work. Parallel work in one package causes merge
conflicts.

## 4. Offer

Ask the user with AskUserQuestion. Offer the top three. Each option has:

- Label: `#<n> [P<x>] <short title>`.
- Description: one line on why it is ready and what it unblocks
  (for example: "unblocks #14 and #15").

If nothing is ready, say why: list the blocked issues and their blockers.

## 5. Hand off

Start the `implement-issue` skill with the chosen issue number.
