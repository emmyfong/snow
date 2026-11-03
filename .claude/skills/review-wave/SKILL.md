---
name: review-wave
description: Run a coordinated multi-reviewer review of a Snow epic branch before its PR exists, or of one or two open pull requests; verify and curate the findings, and write one two-part report (human, then agent). Use before opening any epic PR (issue-handler calls it), when the user asks for a review wave or team review, and for a re-review of a branch or PR that already has a report.
---

# Review wave

Several `reviewer` subagents review the same change in parallel, each with
one focus. You are the main reviewer: you verify, curate, and write one
report. The change is either an epic branch before its PR (the usual case,
from `issue-handler`) or an open PR.

## Environment and scope

Use the active environment's question and delegation tools. Offer reviewer models
that are actually available. Do not invent unavailable tools or model IDs. Honor review settings the
user already supplied. For a branch without a PR, use its branch and SHA in the
review brief and report instead of a nonexistent PR number.

For documentation or skill changes, focus on instruction conflicts, user intent,
references, tool compatibility, and scope. For prompt/context/tool-policy changes,
inspect appropriate regression evidence, including clarification and recovery.
Do not require paid live model runs for unrelated changes.

## 1. Resolve review settings

Reuse the user's settings from the current conversation or handoff. Record the
main reviewer, subagent model, and reviewer count in the report. Ask only for
missing choices, using the available question tool or a concise question in chat.
A saved recommendation or an unanswered question is not a user selection.

Keep review preferences separate from model IDs. The user changes providers, so
discover the current host's delegation capabilities and available models before
resolving reviewers. A previous model choice is reusable only if it is available
through the current environment and still matches the user's intended account
and usage. Do not install or switch providers to reproduce an old review setup.
Prefer a current-provider option when recommending reviewers. Ask about a model
only when a meaningful choice remains; do not require a permanent model preference.
If delegation is absent, explain the available review approach and its independence
limits. Do not invent an equivalent reviewer or claim a review wave ran.

Recommend the current session as main reviewer and an available model for
subagents. Explain any meaningful cost or capability tradeoff. Offer 2, 3, or 4
reviewers with a recommendation suited to the change. Use the host's supported
model-switch mechanism if the user selects a different main reviewer.

If delegation or a selected model is unavailable, explain the limitation and
agree on an available alternative. Do not silently substitute a model or describe
a single-agent review as an independent review wave.

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
gh pr view <pr> --json number,title,body,headRefOid,baseRefName,files,state
gh pr diff <pr> > .context/reviews/pr-<pr>.diff
gh issue view <issue> --json title,body   # each linked issue
```

For a merged PR, report its state and stop before launching reviewers.

Fetch native closing-issue links with the GraphQL query in `.claude/skills/next-issue/SKILL.md`
when needed; do not infer complete issue linkage from the PR body alone.

Record the head SHA. Every finding refers to it. The change usually
completes an epic and its sub-issues. Read the acceptance criteria of every
one. Read the ADRs in `docs/decisions/` that the change
touches.

Run the applicable checks in `references/local-gate.md` before review. Use its
documentation checks for documentation-only changes. Report unavailable checks
explicitly. Fix applicable failures before the reviewers start.

Name the report `<branch-with-dashes>-<sha7>.md` for a branch and
`pr-<n>-<sha7>.md` for a PR. If a report exists for this branch or PR in
`.context/reviews/`, this is a re-review.
Go to section 6 first.

## 3. Launch the reviewers

Fill `templates/reviewer-brief.md` once per reviewer. For code changes, distribute
these concerns across the chosen reviewers:

1. Correctness and concurrency.
2. Cross-platform behavior and tests.
3. API design and conformance with `AGENTS.md`.
4. Security and failure handling where relevant.

For documentation and skills, assign user intent and instruction consistency to
one reviewer, and workflow correctness, references, and tool compatibility to
another. Additional reviewers can examine realistic use cases and scope coverage.
Combine relevant concerns when there are fewer reviewers; do not drop a material
concern solely because of reviewer count.

Launch reviewers with the available delegation tool and the selected model. Give
each the artifact, exact SHA, issue scope, and review brief. Require read-only work.
If the host supports a reviewer role, use it; otherwise put those instructions in
the task. Reviewers inspect the artifact independently before findings are curated.

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
the branch or PR path selected in section 2. Rules:

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

For a branch: show the user the report and ask through the available question tool or chat which
findings to fix. The `issue-handler` session fixes them, each with a test
where behavior changes, runs the local gate again, then opens the PR. Do not
open the PR or say it is ready while a gate check fails. After the PR exists, offer to post
the report as its first comment.

For a PR, or once the branch's PR exists: post only after the user approves,
or if the user already said to post:

```sh
gh pr comment <pr> --body-file .context/reviews/<report>.md
```

Read the comment back with `gh pr view <pr> --comments` and check that the
diagrams and code spans survived.

A merged PR needs no review. Tell the user and stop.
