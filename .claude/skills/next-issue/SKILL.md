---
name: next-issue
description: Choose the next epic to work on in Snow. Reads open epics and standalone issues, their priority labels, sub-issues, and blocked-by links, drops blocked or in-progress work, and offers the top three for the user to pick. Use when the user asks what to work on next, or before starting issue-handler without a named epic.
---

# Choose the next epic

Find the open epics that are ready to start, rank them, and let the user pick.
An epic is one PR. A standalone issue (no parent epic, no sub-issues) counts as
an epic of one. This skill reads only. It changes nothing on GitHub.

## 1. Collect

```sh
R=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
mkdir -p .context
gh issue list -R "$R" --state open --limit 200 \
  --json number,title,labels,assignees,body > .context/issues.json
gh pr list -R "$R" --state open --limit 100 \
  --json number,headRefName > .context/prs.json
# Use GraphQL for closing links; this gh version omits them from JSON fields.
gh api graphql -F owner="${R%/*}" -F name="${R#*/}" -f query='
query($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    pullRequests(states: OPEN, first: 100) {
      nodes {
        number
        closingIssuesReferences(first: 100) { nodes { number } }
      }
    }
  }
}' --jq '.data.repository.pullRequests.nodes' > .context/pr-closing.json
git worktree list --porcelain | grep '^branch ' | sed 's#branch refs/heads/##'
```

For each open epic (label `epic`), list its open sub-issues:

```sh
gh api "repos/$R/issues/<epic>/sub_issues" --jq '.[] | select(.state=="open") | .number'
```

An open issue that is in no epic's list and has no sub-issues is standalone.

For each epic, its sub-issues, and each standalone issue, list open blockers:

```sh
gh api "repos/$R/issues/<n>/dependencies/blocked_by" --jq '.[] | select(.state=="open") | .number'
```

## 2. Filter

First check the body and native links for parked or superseded scope, conflicting
acceptance criteria, and dependency cycles. Report inconsistencies; do not silently
repair GitHub state or treat an open issue as ready work. Honor an explicit user
choice of goal before default priority ranking.

Drop an epic or standalone issue when any of these is true:

- It is parked or superseded and the user has not asked to reactivate it.
- It has an open blocker outside itself. A sub-issue blocked by another
  sub-issue of the same epic does not count; that only sets the build order.
- Every one of its open sub-issues is blocked from outside the epic.
- It has an assignee other than the user.
- An open PR closes it (`closingIssuesReferences`).
- A branch is linked to it on GitHub (`gh issue develop --list <n> -R "$R"`
  prints a branch).
- A local branch for it exists in a worktree (branch `<type>/<n>-<slug>`).

## 3. Rank

1. Priority label: `Priority: Critical`, then `High`, `Medium`, `Low`. An epic
   with no priority label ranks last. Report it as a defect.
2. Then how many other epics or issues it unblocks. More first.
3. Then the lower issue number.

Then prefer independence. If work is in progress in a worktree, move a
candidate down one place when its body names the same `internal/<pkg>`
packages as that work. Parallel work in one package causes merge conflicts.

## 4. Offer

Ask through the available question tool or chat. Offer up to three ready choices.
If the user already selected the work, retain that choice. Each option has:

- Label: `#<n> <short title>` and its priority, for example
  `#12 Shell in a pane (High)`.
- Description: the number of open sub-issues, and what it unblocks (for
  example: "4 sub-issues; unblocks #20").

If nothing is ready, say why: list the blocked epics and their blockers.

## 5. Hand off

Start the `issue-handler` skill with the chosen epic or issue number.
