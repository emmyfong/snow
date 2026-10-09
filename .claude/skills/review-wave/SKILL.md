---
name: review-wave
description: Run a coordinated multi-reviewer review of a Snow epic branch before its PR exists, or of one or two open pull requests; verify and curate the findings, and write one two-part report (human, then agent). Use before opening any epic PR (issue-handler calls it), when the user asks for a review wave or team review, and for a re-review of a branch or PR that already has a report.
---

# Review wave

Several `reviewer` subagents review the same change in parallel, each with
one focus. You are the main reviewer: you verify, curate, and write one
report. The change is either an epic branch before its PR (the usual case,
from `issue-handler`) or an open PR.

## 1. Ask the user

Use AskUserQuestion, one call, three questions:

- The main reviewer model: this session's model, or another. If another, tell
  the user to switch with `/model` and continue in that session.
- The reviewer subagent model: `opus`, `sonnet`, or `haiku`. Recommend
  `sonnet`.
- The number of reviewers: 2, 3, or 4. Default 3.

## 2. Collect the artifact

For a branch before its PR, run in the worktree:

```sh
mkdir -p .context/reviews
git fetch origin
git rev-parse HEAD                                  # the reviewed SHA
git diff origin/main...HEAD > .context/reviews/<branch-with-dashes>.diff
gh issue view <epic> --json title,body              # and each sub-issue
```

For an open PR:

```sh
mkdir -p .context/reviews
gh pr view <pr> --json number,title,body,headRefOid,baseRefName,files,closingIssuesReferences
gh pr diff <pr> > .context/reviews/pr-<pr>.diff
gh issue view <issue> --json title,body   # each linked issue
```

Record the head SHA. Every finding refers to it. The change usually
completes an epic and its sub-issues. Read the acceptance criteria of every
one. Read the ADRs in `docs/decisions/` that the change
touches.

Name the report `<branch-with-dashes>-<sha7>.md` for a branch and
`pr-<n>-<sha7>.md` for a PR. If a report exists for this branch or PR in
`.context/reviews/`, this is a re-review.
Go to section 6 first.

## 3. Launch the reviewers

Fill `templates/reviewer-brief.md` once per reviewer. Give each reviewer one
focus, in this order:

1. Correctness and concurrency.
2. Cross-platform behavior and tests.
3. API design and conformance with `AGENTS.md`.
4. Security and failure handling (only with four reviewers).

Launch all of them in one message, in parallel, with the Agent tool:
`subagent_type: reviewer`, `model: <the chosen model>`, and the filled brief
as the prompt. With two reviewers, give the second reviewer foci 2 and 3.

## 4. Verify and curate

Reviewer findings are claims, not facts.

- For each finding, open the cited code and check the claim. Run a narrow test
  or command if a claim depends on behavior.
- Drop findings that are wrong, duplicated, out of scope, or too small to
  matter.
- Merge duplicates from different reviewers into one finding.
- Keep a small documentation fix when the current text misleads agents or
  users. A one-line fix to a rule that many agents follow can be high impact.
- Grade each kept finding with `references/severity.md`.

## 5. Write the report

Fill `templates/report.md`. Save it to
`.context/reviews/pr-<pr>-<sha7>.md`. Rules:

- Human part first, then agent part.
- Human part: short bullets, one table, a few ASCII diagrams, glossary last.
- Every blocker gets an ASCII diagram of the fix. A non-obvious fix gets one
  too.
- Every finding has a recommended fix and at least one alternative with its
  tradeoff.
- Use industry-standard terms or terms from the codebase. Do not invent terms.
- ASD-STE100 style. No em-dashes.

Print the absolute path of the report. Ask the user to approve it.

## 6. Re-review

Load the previous report. Walk every finding against the new head SHA. Mark
each one `fixed`, `open`, or `regressed`, with evidence. Then review the new
changes since the old SHA as in sections 3 to 5. The new report keeps the
old checklist at the top of the agent part.

## 7. Fix, then post

For a branch: show the user the report and ask, with AskUserQuestion, which
findings to fix. The `issue-handler` session fixes them, each with a test
where behavior changes, then opens the PR. After the PR exists, offer to post
the report as its first comment.

For a PR, or once the branch's PR exists: post only after the user approves,
or if the user already said to post:

```sh
gh pr comment <pr> --body-file .context/reviews/<report>.md
```

Read the comment back with `gh pr view <pr> --comments` and check that the
diagrams and code spans survived.

A merged PR needs no review. Tell the user and stop.
