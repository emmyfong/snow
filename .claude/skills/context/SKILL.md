---
name: context
description: Write or refresh .context/CONTEXT.md, the working memory and handoff note for this working copy. Use after a milestone or commit, when the user says "handoff", "save context", or "where are we", when the user switches model or effort, and when a PR is done (to graduate notes into AGENTS.md or an ADR).
---

# Keep working memory in .context/CONTEXT.md

`CONTEXT.md` lets a new session, a new model, or a parallel agent continue the
work without rereading the session history. A `SessionStart` hook prints it at
startup, resume, clear, and after compaction.

## Where the file lives

- One file per working copy: `$(git rev-parse --show-toplevel)/.context/CONTEXT.md`.
- Each worktree has its own file. Never write another worktree's file.
- `.context/` is gitignored. Create the folder if it is missing.

## When to write it

- After a milestone: a passing step, a commit, a finished sub-task.
- When the user says "handoff", "save context", or switches model or effort.
  Write the file before you answer anything else.
- Before you stop for the day or hand back to the user with work unfinished.
- Compaction can happen at any time. Do not let the file go stale for long.

## How to write it

- Rewrite the whole file. Never append. It is a snapshot, not a log.
- Keep it under 100 lines. Cut stale items first: finished steps that no
  longer matter, gotchas that no longer apply.
- Write facts, not narration. File paths, issue numbers, commands, and
  measured results beat descriptions.
- Do not copy code into it. Point to the file and line.

## Template

```markdown
# Context: <short goal>

Updated: <YYYY-MM-DD HH:MM>, branch `<branch>`, by <model>

## Handoff
- Next step: <the exact next action, specific enough to start without asking>
- Verify: <what to check first, e.g. a command and its expected result>
- Open questions for the user: <or "none">

## Goal
<issue #N and one line on what done means>

## State
- Done: <bullets>
- In progress: <bullets>
- Next: <bullets, in order>

## Decisions
- <decision>: <why>. (ADR NNNN if recorded.)

## Gotchas
- <dead end or trap>: <what happened, what to do instead>

## Files that matter
- `<path>`: <why it matters>
```

## Handoff

When the user says "handoff" or switches model or effort:

1. Rewrite the file with the template. Make the Handoff section complete.
2. Tell the user the absolute path and the next step in one or two lines.

## Graduation

When the PR for this working copy is done, read the Decisions and Gotchas
sections and ask the user, with AskUserQuestion, which items must outlive the
worktree:

- A rule every agent must follow goes into `AGENTS.md` (committed).
- A design choice goes into an ADR with the `decision-record` skill.
- Everything else is deleted with the worktree.
