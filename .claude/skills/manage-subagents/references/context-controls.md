# Verify context controls

The user requested automatic compaction at 40% of context. Treat this as a runtime
configuration requirement, not a prompt instruction. Keep handoffs regardless of
whether the runtime exposes the requested controls.

## Before launch

1. Inspect the active host's supported configuration and spawn arguments.
2. Determine the effective context window for each selected model from trusted
   runtime metadata. Do not invent a window size or assume all models share one.
3. If the host accepts a token threshold for full active context, calculate
   `floor(0.40 * effective_context_window_tokens)`. Confirm the counted scope.
4. Apply the supported setting at the narrowest authorized scope before launch.
   Preserve unrelated settings. Verify the effective value and child inheritance.
   If a restart is required, report it; do not claim the running child changed.
5. Record the model, context window, threshold, counting scope, verification source,
   and whether automatic enforcement is confirmed or unavailable.

If the host exposes neither a threshold nor child settings, say that 40% automatic
compaction cannot be enforced in this run. Continue authorized work with bounded
briefs and milestone handoffs. Do not label that fallback automatic compaction.
If context telemetry is missing, do not invent a percentage from turn counts or
text length. Use milestone checkpoints, and disclose that the threshold is unknown.

## Codex example, verified 2026-11-12

The [official configuration reference](https://developers.openai.com/codex/config-reference/)
documents these distinct controls:

- `model_auto_compact_token_limit`: automatic compaction threshold in tokens.
- `model_auto_compact_token_limit_scope`: `total` counts full active context;
  `body_after_prefix` counts growth after the carried prefix.
- `model_context_window`: the active model's context window in tokens.
- `model_post_turn_compact_threshold_percent`: compaction after a final response.
  This is not continuous threshold enforcement during a long turn.

Check the installed host and version before applying any of these. A documented
configuration key does not prove a hosted spawn tool accepts it or that children
inherit it. Do not change the advertised context window to manufacture a percentage.
Do not change global settings when a task-scoped setting will suffice.

Some Claude Code setups use `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE`. That is not a Codex
spawn argument. Verify the relevant host's current documentation and effective
configuration before using a host-specific setting. Never copy a setting across
hosts based on its name alone.
