# Skill format

## Folder

```
.claude/skills/<name>/
  SKILL.md          required; the procedure
  references/       optional; detail that SKILL.md links to
  templates/        optional; files an agent copies and fills
```

The folder name equals `name` in the front matter. Do not copy a skill into an
agent-specific folder. Every agent reads this one copy by path.

## SKILL.md skeleton

```markdown
---
name: <kebab-case>
description: <What it does, in one sentence.> Use when <trigger>, when <trigger>, and when <trigger>.
---

# <Title in plain words>

<Two to four sentences: the goal, and what done looks like.>

## <Preconditions, if any>

## 1. <Imperative step>
## 2. <Imperative step>

## Improve this skill

When a step here was wrong, missing, or unclear, propose a correction through
`compose-skill`. Obtain user approval before editing unless that change is already
authorized. Use a dedicated skill issue, worktree, and PR. Continue independent
task work while approval is pending.
```

## Rules

- **Description first.** An agent sees only the description when it chooses a
  skill. Put the triggers in it: user phrases, situations, and the skills that
  hand off to this one.
- **Steps, not essays.** Each step starts with a verb. A rule that applies to
  every step goes in a short list before step 1.
- **Exact commands.** Give the literal `gh`, `git`, or `go` command. Name files
  by path. Name tests by function name.
- **Gates are explicit.** If a step needs the user's approval, say so in that
  step and say what to show the user.
- **Hand-offs are explicit.** Name the next skill and what to pass to it.
- **No duplication.** Do not restate `AGENTS.md` rules. Link to the section.
- **Tool-neutral.** Describe the action. A host-specific tool name may appear
  as an example only.
- **Style.** ASD-STE100: short sentences, one idea each, active voice, plain
  words, no em-dashes. Use terms from the codebase.

## Checklist

- [ ] The description names the triggers.
- [ ] Every step can run on Claude Code, Codex, and OpenCode.
- [ ] No rule contradicts `AGENTS.md` or another skill.
- [ ] Under about 200 lines; long detail is in `references/`.
- [ ] Ends with "Improve this skill".
- [ ] `grep -rn '—' .claude/skills/<name>/` prints nothing.
- [ ] The `AGENTS.md` catalog lists the skill with a one-line purpose.
- [ ] A changelog entry records the evidence and the test.
