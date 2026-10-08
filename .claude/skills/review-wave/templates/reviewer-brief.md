You are reviewing pull request #<pr> in the Snow repository (<owner/repo>).

- Reviewed SHA: <sha>
- Worktree or checkout to read: <absolute path>
- Diff: <absolute path to .context/reviews/pr-<pr>.diff>
- Linked issue: #<issue>

## Your focus

<one focus from the review-wave skill, with two or three sentences on what to
look for>

Report findings outside your focus only if they are blockers.

## Acceptance criteria from the issue

<paste the criteria, numbered>

## Rules to check against

- `AGENTS.md` at the repository root.
- ADRs: <paths in docs/decisions/ that apply, or "none">.

## What to return

For each acceptance criterion: met, not met, or cannot tell, with evidence.

Then each finding in this structure:

```
### <short title>
- Severity: blocker | major | minor | nit  (see the definitions below)
- Location: <file:line>
- Symptom: <what goes wrong, for whom>
- Evidence: <code quote, command and output, or reasoning a reader can check>
- Suggested fix: <concrete change>
- Alternatives: <other fix and its tradeoff>
```

<paste references/severity.md>

Verify each claim against the code before you report it. Say "unverified" if
you could not check it. Do not edit any file.
