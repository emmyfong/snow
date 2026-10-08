---
name: review-wave
description: Run a coordinated multi-reviewer review of one or two Snow pull requests, verify and curate the findings, and write one two-part report (human, then agent) to post as a PR comment after user approval. Use when the user asks for a review wave, a team review, or a curated review report on a PR. Also use for a re-review of a PR that already has a review-wave report.
---

# Review wave

Several `reviewer` subagents review the same PR in parallel, each with one
focus. You are the main reviewer: you verify, curate, and write one report.

## 1. Ask the user

Use AskUserQuestion, one call, three questions:

- The main reviewer model: this session's model, or another. If another, tell
  the user to switch with `/model` and continue in that session.
- The reviewer subagent model: `opus`, `sonnet`, or `haiku`. Recommend
  `sonnet`.
- The number of reviewers: 2, 3, or 4. Default 3.

## 2. Collect the artifact

```sh
mkdir -p .context/reviews
gh pr view <pr> --json number,title,body,headRefOid,baseRefName,files,closingIssuesReferences
gh pr diff <pr> > .context/reviews/pr-<pr>.diff
gh issue view <issue> --json title,body   # each linked issue
```

Record the head SHA. Every finding refers to it. The PR usually closes an
epic and its sub-issues. Read the acceptance criteria of every closed issue. Read the ADRs in `docs/decisions/` that the change
touches.

If a report exists for this PR in `.context/reviews/`, this is a re-review.
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

## 7. Post

Post only after the user approves, or if the user already said to post:

```sh
gh pr comment <pr> --body-file .context/reviews/pr-<pr>-<sha7>.md
```

Read the comment back with `gh pr view <pr> --comments` and check that the
diagrams and code spans survived.

A merged PR needs no review. Tell the user and stop.
