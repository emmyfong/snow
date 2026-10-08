@AGENTS.md

## Claude Code notes

- Snow's project skills (`.claude/skills/`) take precedence over superpowers skills for issue, review, commit, and context workflows. Superpowers stays available for generic work such as brainstorming and debugging.
- Hooks in `.claude/hooks/` enforce the git rules. If a command is blocked, do not work around it. Report it to the user.
- Use the `context` skill for handoffs and when the user switches model or effort.
