---
name: plan-program
description: Plan a large body of Snow work (many epics across several areas) with a manager agent that splits it into workstreams, delegates research and drafting to subagents, checks every draft with an independent verifier, and brings each workstream to the user for approval through compose-issues. Use when the user asks to plan the whole project, a release, or several epics at once, or asks for a team of planning agents.
---

# Plan a program of epics

The manager (you) owns the plan and talks to the user. Subagents research and
draft. A fresh verifier checks every draft against the code before you see it.
The output is implementer-ready epics and sub-issues that other agents build
alone. Planning never authorizes implementation.

## Roles

- **Manager.** Splits the program, writes briefs, curates drafts, keeps one
  glossary, asks the user, publishes through `compose-issues`.
- **Drafter.** One per workstream. Reads the code, issues, ADRs, and
  `DESIGN.md`. Writes the structure, then the bodies. Never publishes.
- **Verifier.** A new subagent for each check, with no drafter context. Runs
  the readiness check and returns pass or a numbered fix list. Never edits.

Use the subagent model the user chose. On Claude Code, the `planner` and
`plan-verifier` agents in `.claude/agents/` pin it. On other hosts, give the
same briefs to that host's subagents.

## 1. Set up the program

1. Record the scope, the user's decisions, and their dates in
   `.context/compose/program/PROGRAM.md`. Record the planning commit:
   `git fetch origin && git rev-parse origin/main`.
2. Collect the inventory once, for every drafter to read:
   ```sh
   mkdir -p .context/compose/program
   gh issue list --state open --limit 300 --json number,title,labels,body > .context/compose/program/issues.json
   ```
3. Split the program into workstreams. A workstream is a user-visible area with
   few shared packages. List the existing epics and issues each one owns. Give
   each issue to exactly one workstream.
4. Start `.context/compose/glossary.md` from terms in the code and `DESIGN.md`.
5. Show the user the workstreams and their order, unless they already approved
   them.

## 2. Draft the structure (per workstream)

1. Fill `templates/drafter-brief.md` with phase `structure`, and start the
   drafter. Run independent workstreams in parallel; respect the host's limit
   on concurrent subagents.
2. The drafter writes `.context/compose/program/<ws>/STRUCTURE.md`: epics,
   sub-issues, labels, the DAG, re-homed and retitled issues, cross-workstream
   arrows, and open product questions with a recommendation for each.
3. Start the verifier with `templates/verifier-brief.md`. It checks the
   structure rules in `compose-issues` step 2 and that every cited issue and
   path exists.
4. Loop: send the fix list to the drafter (continue the same drafter if the
   host allows it). Stop at pass, or after three rounds. Then fix the rest
   yourself, and ask the user about any product choice. Write each fix list
   to a file and send its path, so the drafter and the next verifier read the
   same text.
5. A verifier often cannot open a page that needs a sign-in, such as a design
   canvas. Read it yourself and put the values in the brief. Reject a fix that
   only says "add a blocker until the page is read".

## 3. Curate and ask

1. Read every passed structure yourself. Check overlaps between workstreams:
   two epics that edit the same package in parallel need an arrow or a merge.
2. Merge open product questions from all workstreams into
   `.context/compose/program/QUESTIONS.md`. Ask the user in small batches, with
   the tradeoff and a recommendation for each. Reuse earlier answers.
3. Present one workstream's structure at a time, as `compose-issues` step 3
   requires. Apply corrections and update dependent workstreams.

## 4. Draft the bodies (per approved workstream)

1. Start the drafter with phase `bodies`. It writes one file per issue in
   `.context/compose/program/<ws>/`, with the sections in
   `compose-issues/references/body-sections.md` and
   `compose-issues/references/implementer-ready.md`.
2. Verify each body with a fresh verifier. Loop as in step 2.4.
3. Curate, then present the bodies for approval (`compose-issues` step 5).
   For a large workstream, publish a page (on Claude Code, an Artifact) with
   each epic, its sub-issues, priorities, and blockers, the user's decisions,
   and a numbered list of the defaults the drafters chose. Ask about the
   defaults that change behavior. Let the user name numbers to change.
4. If an answer changes a decision that bodies already use, rework every body
   that depends on it and verify those bodies again before you publish.

## 5. Publish

Publish approved workstreams with `compose-issues` steps 6 to 8. Keep the
draft-to-number mapping in `.context/compose/program/PUBLISHED.md`, so a later
workstream can link to earlier issues. For more than a few issues, write a
manifest (key, body file, issue number or new, title, labels, parent,
blockers) and a script that records each finished operation in a state file.
A rerun then skips the done work. Report which epics are ready for
`issue-handler` and which are blocked.

## Rules

- A subagent's report is a claim. Check each fact that the plan depends on.
- A drafter never decides a product question. It writes the question and a
  recommendation. A default it picks for user-visible behavior (a label, a
  button set, a permission scope) is a question too. It marks the line
  "Recommendation, user confirms at bodies approval".
- Drafters and verifiers do not run the programs the plan is about: no
  probes, logins, or servers. A fact that needs a probe becomes a step in the
  issue, with a stop rule. The implementer runs it with the user's approval,
  in an isolated profile (for example, a temporary `CODEX_HOME`), and never
  starts a machine-wide service. These limits are instructions only; the host
  does not enforce them. After each drafter, check `git status` and the
  repository's recent issue activity.
- Prefer one way to do a thing. A second mode, preset, or compatibility rule
  needs a reason from the user.
- Keep `.context/CONTEXT.md` current with the `context` skill after each
  approved workstream, so a compaction or a new session can continue.
- Keep briefs self-contained: a subagent sees only its brief and the files it
  opens.

## Improve this skill

When a step here was wrong, missing, or unclear, propose a correction through
`compose-skill`. Obtain user approval before editing unless that change is already
authorized. Use a dedicated skill issue, worktree, and PR. Continue independent
task work while approval is pending.
