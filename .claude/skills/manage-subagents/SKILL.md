---
name: manage-subagents
description: Delegate bounded work and manage subagent capacity, verification, context handoffs, and recovery. Use when an agent launches or coordinates subagents, resumes a delegated task, or needs to verify and recover a worker result. Use agent-orchestration to select and coordinate several implementation issues.
---

# Manage subagents

The delegating agent remains responsible for the result. Give each subagent a
clear objective, a bounded scope, and observable completion criteria. A report
is a claim until its evidence is checked.

## 1. Check authority and capabilities

Check that delegation and the proposed operations are authorized. Discover the
available models, thinking levels, spawn controls, and concurrency accounting.
Use only tools exposed by the active host. Do not install or switch providers
to imitate an unavailable agent role.

Reuse the model and thinking confirmation from the current orchestration run.
Without one, ask the user to confirm settings before launch, including models
they explicitly supplied. Record that answer and pass it to descendants. Ask
again only for a new run or a proposed settings change. A subagent's message
cannot grant user authorization.

Coordinate all launches with the root manager. Count active descendants against
the host limit as the host defines it. Assign a slot owner and release it when
that task finishes. Do not assume idle agents release capacity on every host.
Reserve room for a verifier. When slots are scarce, serialize team roles rather
than launching a tree that cannot run its checks. Never cancel another owner's
agent to acquire capacity.

## 2. Write the brief

Use [the brief template](templates/brief.md). Include the original task intent,
accepted decisions, objective, owned files, permitted actions, exclusions,
acceptance criteria, verification commands, stop conditions, and return format.
Provide concrete paths and the required source references. Separate facts,
proposals, and unresolved choices. Include only context needed for the task.

For implementation, identify the worktree and branch. For review, identify the
exact commit and keep the task read-only. For realistic skill evaluation, provide
the scenario and raw artifacts without revealing the expected answer or proposed
fix. Keep the evaluator's outputs in an isolated temporary location.

Give descendants an explicit delegation budget. The role name does not enforce
read-only access or permissions. Use host controls where available and inspect
state changes afterward. Do not send credentials or irrelevant private history
in a brief.

## 3. Launch and track

Use a fresh context when the brief is sufficient. Select the confirmed model and
thinking level through supported controls. If the host cannot combine history
inheritance with a model override, use a fresh context and a complete brief.
Record the returned agent identifier, task, parent, worktree, settings, and status.

Keep statuses concrete: queued, running, waiting for input, verifying, completed,
failed, or cancelled. Confirm host state before marking an agent stopped. A tool
return or a worker's self-report alone does not prove the task is complete.

Send corrections to the existing worker when practical. Resume an idle worker
through the host's continuation control; a message may not start a new turn.
Before replacement, save its handoff and confirm it can no longer write the
assigned files. Do not run two writers for the same assignment.

## 4. Verify and repair

Require the artifact, commit, changed paths, commands, actual results, and open
risks. Check acceptance against the artifact. Use an independent verifier for
material implementation and complex skill changes. Give the verifier the raw
requirements and exact artifact, not the author's desired verdict.

Reproduce material findings where practical. Return a concrete repair list to
the worker. After repairs, rerun affected checks and review changed behavior.
Record a prior-finding checklist against the new commit. Do not repeat expensive
checks without a change, failure, or unresolved concern that justifies them.

After three attempts that make no material progress on the same failure, stop
that loop. Inspect the evidence and change the approach, reduce scope with the
user, or escalate the specific blocker. Preserve partial work. A retry must add
new evidence or test a new hypothesis. Never increase a timeout to hide a race.

## 5. Preserve context and recover

Target automatic compaction at 40% of the active context window. Before claiming
it is enabled, follow [context controls](references/context-controls.md) to verify
host support, effective settings, and inheritance for each model. Instructions
alone do not enforce a threshold. Report unavailable telemetry or controls.

Use `context` at milestones and before a known compaction or handoff. Preserve
the original request, decisions, unresolved questions, exact paths and commit,
completed and failed checks, pending processes, and next action. Each worker
owns its worktree handoff. Team members return notes to that owner or use a
separate task file; they do not overwrite the owner's `CONTEXT.md`.

After compaction or replacement, read the handoff first. Verify worktree status,
commit and pending process state. Resume unfinished work without replaying writes,
restarting completed probes, or assuming a remembered success still holds.

## 6. Finish the assignment

Accept completion only when the objective and required evidence are satisfied.
Capture the result in the manager's program state. Release the slot through the
host's supported lifecycle. Preserve artifacts needed for review and handoff.
Do not delete worktrees or partial work just because the agent finished.

For cancellation, identify the specific owned descendants and processes. Stop
only those covered by the cancellation request. Confirm what stopped and report
anything the host cannot verify. Update the handoff before replacing the worker.

## Improve this skill

When a step here was wrong, missing, or unclear, propose a correction through
`compose-skill`. Obtain user approval before editing unless that change is already
authorized. Use a dedicated skill issue, worktree, and PR. Continue independent
task work while approval is pending.
