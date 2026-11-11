# Skill changelog

Agents add an entry at the top for every skill change (see `compose-skill`).
The user reviews `pending review` entries in batches.

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
  - New Claude Code agents `planner` and `plan-verifier` pin Haiku 5.5.
- Test: walk-through of "plan the whole of Snow as implementer-ready epics"
  against `plan-program` and `compose-issues`; the first live run is the
  planning program of 2026-11-10.
- Status: pending review
