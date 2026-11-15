---
name: compose-skill
description: Propose improvements to Snow's shared skills, then implement approved changes in a dedicated issue, worktree, and PR with validation. Use when the user asks for a new skill or a skill change, when a skill step failed or the user corrected an agent who followed a skill, and when another skill's "Improve this skill" step sends you here.
---

# Compose and improve skills

A skill is a shared procedure that any supported agent can read by path.
An agent can identify and propose improvements. The user approves the scope
before the agent edits skill instructions. Approved skill work has its own
issue, worktree, and PR.

## Before editing: propose and obtain approval

Use evidence from real work: a failed or ambiguous step, a user correction,
conflicting instructions, or an obsolete command. Do not change skills for taste
or duplicate a rule that already lives in `AGENTS.md`.

Show the user a concise proposal with:

- The observed problem and its evidence.
- The proposed correction and the affected files or skills.
- Any change to behavior, permissions, approval, cost, or publication scope.
- The intended dedicated skill issue, worktree, validation, and PR.

Do not edit skill files before approval. A request to fix task output does not
by itself authorize changing the workflow. Silence and a subagent's suggestion
are not approval. Keep evidence in the current task's working notes and continue
independent authorized work while the proposal is pending. If the user declines,
leave the skills unchanged and record the decision in the task handoff.

An explicit user request to create or change a skill is approval for that scope.
Reuse earlier approval for the same change. Do not add a redundant confirmation.
Resolve only consequential choices that the request leaves open. Revisions needed
to validate an approved change stay in scope; a new policy change needs approval.

## Where to edit

After approval, create or select the dedicated skill issue. Follow `issue-handler`
for its linked branch and separate worktree under `.worktrees/`. Keep the original
implementation worktree unchanged. If already working in the dedicated approved
skill worktree, continue there; do not create recursive worktrees for routine
review fixes. Never edit skills in the main checkout.

Keep the skill files, related shared rules, and changelog in the dedicated PR.
Approval covers this normal issue and PR workflow unless the user limits the
request to exploration, a draft, or local changes. Preserve that limit. Do not
bundle an improvement into an unrelated task PR, even as a separate commit.
Never weaken an approval or authorship rule without explicit authorization.

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

## 5. Review and publish

Run the documentation checks and `review-wave` on the dedicated skill branch.
Resolve verified findings within the approved scope. Record the scenario results
in the changelog. Open the dedicated PR through `issue-handler` when its checks
pass. Use the publication scope recorded before editing: normal implementation
approval includes opening this dedicated PR, unless the user requested a draft,
local-only work, or a final approval before publication. Do not add a new final
approval gate. Report the PR and leave merging to the user.
Do not claim the skill is active on main before it is merged.

The changelog is evidence, not retroactive approval. Keep new entries `pending
review` until the user accepts them. Historical entries describe earlier policy;
they do not override current instructions. When asked for a batch review, show
the pending entries and ask which to keep, change, or revert. Apply approved
changes through the same dedicated workflow.

## Improve this skill

Use this skill's proposal and approval steps for improvements to this skill too.
Do not edit it automatically because it suggested changing itself.
