---
name: compose-skill
description: Create, change, or repair one of Snow's shared skills in .claude/skills/, test it on a realistic scenario, and log the change for the user's batch review. Use when the user asks for a new skill or a skill change, when a skill step failed or the user corrected an agent who followed a skill, and when another skill's "Improve this skill" step sends you here.
---

# Compose and improve skills

A skill is a short, shared procedure that any agent (Claude Code, Codex,
OpenCode) can open by path and follow. Skills improve themselves: an agent that
finds a gap fixes the skill at once and logs the fix. A fix can reach `main`
inside the PR of the task that found it. The user reviews the logged fixes in
batches, and an entry stays `pending review` until then.

## When to change a skill

Change a skill only for evidence from real use:

- A step was wrong, missing, or ambiguous, and an agent lost time or erred.
- The user corrected an agent who followed the skill.
- Two skills contradict each other or `AGENTS.md`.
- A command, path, label, or tool in the skill no longer exists.

Do not change a skill for taste, a one-off task, or a rule that already lives in
`AGENTS.md`. Never weaken a safety, authorship, or approval rule. Ask the user
before you add an approval gate, remove one, or widen what a skill may publish.

## Where to edit

Skill files are committed. Edit them in a worktree on a branch, never in the
main checkout. If your task has a worktree, edit there: the fix ships in that
task's PR (user decision, 2026-11-12). Keep it in its own `docs(skills):`
commit, and name it under "Skill changes" in the PR body. Without a task
worktree, use the open skills branch the user named, or ask for one.

## 1. Find the right home

- Read `AGENTS.md` (the skill catalog) and the skills the change touches.
- Prefer a change to an existing skill over a new one. Add a skill only for a
  workflow that recurs and that no skill covers.
- One skill owns one workflow. Move shared detail into that skill's
  `references/`, and link to it.

## 2. Write it in the house format

Follow `references/skill-format.md`. In short:

- Front matter: `name` (kebab case, equal to the folder) and `description`. The
  description says what the skill does and lists its triggers ("Use when ...").
  Agents choose skills by the description alone.
- Body: numbered steps in imperative voice, ASD-STE100 style, no em-dashes.
- Under about 200 lines. Move long tables, templates, and examples into
  `references/` or `templates/`.
- Tool-neutral: name the action ("ask the user", "delegate to a subagent"),
  not one host's tool. Commands use `gh`, `git`, and `go`.
- End with the "Improve this skill" section from the format file.

## 3. Test the change

Pick one realistic scenario: the situation that exposed the gap, or for a new
skill, its most common use. Then:

1. Write the scenario and the expected agent behavior in two to five lines.
2. Read the changed skill as a new agent would, with only `AGENTS.md` and the
   skill. Walk each step against the scenario.
3. Check that each step can be done with the tools of Claude Code, Codex, and
   OpenCode, and that no step depends on this session's memory.
4. If you can delegate, give a fresh subagent the scenario and the skill path,
   and compare what it plans to do with the expected behavior.

Fix the skill until the walk-through reaches the expected behavior. Record the
scenario and the result in the changelog entry.

## 4. Log the change

Add one entry to the top of `.claude/skills/CHANGELOG.md`:

```markdown
## <YYYY-MM-DD> <skill>: <one-line change>
- Evidence: <what failed or what the user said, with issue or PR numbers>
- Change: <what the skill now says>
- Test: <scenario, and the result of the walk-through>
- Status: pending review
```

Keep the changelog, the skill files, and the `AGENTS.md` catalog in one
commit: `docs(skills): <summary>`.

## 5. Batch review

The user reviews pending entries in batches. When asked to prepare a batch:

1. List every `pending review` entry with its diff summary and where it is
   (on `main` through a PR, or on an open branch).
2. Ask the user which entries to keep, change, or revert.
3. Apply the answers and set kept entries to `Status: accepted`. Open one PR
   for the batch. Ask before a docs-only PR, as `AGENTS.md` requires.

## Improve this skill

If this skill failed you, fix it with itself: follow steps 1 to 4 on
`compose-skill`.
