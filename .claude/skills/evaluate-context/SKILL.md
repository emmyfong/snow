---
name: evaluate-context
description: Plan or assess a Snow prompt, context, caching, compaction, tool-output, or agent-coordination change using task quality and total usage. Use for efficiency experiments; not for unrelated code or documentation edits.
---

# Evaluate context changes

Read the relevant efficiency and quality sections of DESIGN.md, the issue, and
the production path affected. Identify the measured problem before selecting an
optimization. If there is no baseline, propose the smallest useful measurement.

## Establish control and evidence

State whether Snow controls the input, a Snow tool, the direct-API loop, an external
runtime setting, or a gateway policy. Record runtime/model versions and relevant
settings. Do not infer internal context or cache hits from visible transcript size.
Keep unknown telemetry explicit. API estimates and subscription allowance differ.

Define a falsifiable hypothesis, a baseline, a candidate, and independent success
criteria. Include intent preservation, missed constraints, useful clarification,
rework, latency and total observable usage. Count preparation, summaries, child
agents and retries. Do not add cumulative usage snapshots together as increments.

## Select representative tasks

Use tasks that can expose the proposed failure mode. Examples include stale files
after reuse, a critical error outside a log excerpt, a user correction before
compaction, restart with an unresolved question, or parallel agents editing the
same resource. Compare delegation with a sequential baseline when that is the
hypothesis. Avoid benchmarks that merely reproduce the implementation's wording.

Hold task information and settings constant when comparing formats or policies.
Keep original prompts and provenance. A JSON transport envelope is not a JSON
prompt requirement. A smaller context that loses user intent is not a win.

## Run only the authorized experiment

Start with deterministic checks for accounting, persistence, filtering and protocol
behavior. Live model evaluations need an agreed budget and connection. Use prior
authorization where available; otherwise prepare the runnable experiment and ask
only for the missing budget or consequential setting. Do not change accounts,
models, routes or effort silently to complete an evaluation.

For variable model trials, record repeated outcomes and uncertainty. Keep some
tasks separate from examples used to tune the policy. A runtime's claimed success
is not independent evidence that the task succeeded.

## Report a decision

Report what changed, what was measured, quality and usage outcomes, missing data,
and reproducible settings. Recommend adoption, rejection, or a specific next
experiment. Do not turn results from one runtime or task into a universal saving.
File implementation work through the existing issue workflow only when warranted.

## Improve this skill

When a step here was wrong, missing, or unclear, or the user corrected you while
you followed it, fix this skill now with the `compose-skill` skill. Then continue
your task.
