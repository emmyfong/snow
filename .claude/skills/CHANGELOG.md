# Skill changelog

Agents add an entry at the top for every skill change (see `compose-skill`).
A fix may reach `main` in the PR of the task that found it. The user reviews
`pending review` entries in batches.

## 2026-11-12 compose-skill: approve improvements before separate skill work
- Evidence: user revised the immediate-edit and same-task-PR policy after #354. Tracked in #359.
- Change: require a concrete proposal and approval before agent-initiated edits; honor explicit requests; use a dedicated issue, worktree, and PR. Align all current Improve this skill sections and remove the issue-handler exception.
- Test: fresh Luna walkthrough kept unapproved discoveries unchanged, honored explicit authorization, respected refusal, continued independent work, and separated routine review fixes from new permission scope. Clarified publication scope after the walkthrough; structural validation passed.
- Status: pending review

## 2026-11-12 compose-skill, issue-handler: a skill fix ships in the task's PR
- Evidence: the #353 review found that "edit in your task's worktree"
  conflicted with the batch review and with the issue-handler scope rule. The
  user chose on 2026-11-12: a skill fix ships inside the epic PR.
- Change: `compose-skill` "Where to edit" puts the fix in its own
  `docs(skills):` commit on the task branch and names it under "Skill changes"
  in the PR body. The batch review lists where each entry is. `issue-handler`
  allows this one exception to its scope rule. `AGENTS.md` says the same.
- Test: scenario: an issue-handler session finds a wrong `tui` step. Walk-through
  result: the agent fixes the step in its worktree, commits it apart from the
  feature commits, logs it here, and lists it in the PR body. No step sends it
  to another branch.
- Status: pending review

## 2026-11-12 compose-issues: prepare bodies and check links before publishing
- Evidence: publishing 77 bodies on 2026-11-11 found planning headers,
  revision notes, bare planning keys, and local file paths in the bodies. One
  `gh issue create` returned a GraphQL error after it created the issue.
- Change: step 6 strips headers, posts a `## Revision` section as one comment,
  resolves bare and existing-issue keys, and replaces local file references. It
  checks for a duplicate after a failed create, and checks the parent and the
  current blockers before linking. Step 7 compares each posted body with its
  prepared file exactly and counts native sub-issues.
- Test: scenario: publish a workstream with new and existing issues. Result:
  the 2026-11-11 run followed these steps and read back 77 bodies with no
  mismatch and no duplicate issue.
- Status: pending review

## 2026-11-12 plan-program: no probes, marked defaults, one way, safe publishing
- Evidence: in the 2026-11-10 run a drafter ran a Codex probe against the
  user's real `~/.codex`, and an OpenCode command tried to start a machine-wide
  service. Drafters chose user-visible defaults as if decided. The user found two
  key systems (Alt keys and a tmux preset) confusing and chose one.
- Change: drafters and verifiers run only read-only `git`, `grep`, and `gh`;
  implementer probes run in an isolated profile with the user's approval;
  drafter defaults are marked for user confirmation; one way to do a thing;
  fix lists as files; the manager reads pages a verifier cannot open; an
  approval page with numbered defaults; rework and verify again when an answer
  changes a used decision; publish with a manifest and a state file. The
  drafter and verifier briefs carry the same command list.
- Test: scenario: plan one workstream that depends on an installed CLI.
  Result of the walk-through: the drafter writes the probe as an issue step
  with a stop rule and runs nothing; the manager asks about each marked
  default before publishing.
- Status: pending review

## 2026-11-10 compose-skill, plan-program: add self-improving skills and program planning
- Evidence: the user asked for skills that update themselves, a skill
  composer, and a manager session that plans the whole project with drafting
  and verifying subagents, so that Codex and OpenCode can implement the epics
  without questions.
- Change:
  - New `compose-skill`, with `references/skill-format.md`.
  - New `plan-program`, with drafter and verifier briefs.
  - New `compose-issues/references/implementer-ready.md`. `compose-issues`
    step 4 now requires it and points to `plan-program`.
  - Every skill ends with "Improve this skill".
  - `AGENTS.md` lists the two skills and the self-improvement rule.
  - New Claude Code agents `planner` and `plan-verifier` pin Haiku 5.5 (the
    user chose Haiku drafters and verifiers on 2026-11-10).
- Test: walk-through of "plan the whole of Snow as implementer-ready epics"
  against `plan-program` and `compose-issues`; the first live run is the
  planning program of 2026-11-10.
- Status: pending review
