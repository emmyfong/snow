@AGENTS.md

## Shared project skills

The shared catalog is in AGENTS.md. Skill files in `.claude/skills/` also serve
other agents that read them directly. Keep Claude-specific setup in this file.

## Claude Code notes

- Snow's project skills (`.claude/skills/`) take precedence over superpowers skills for issue, review, commit, and context workflows. Superpowers stays available for generic work such as brainstorming and debugging.
- Hooks in `.claude/hooks/` enforce the git rules. If a command is blocked, do not work around it. Report it to the user.
- Use the `context` skill for handoffs and when the user switches model or effort.
