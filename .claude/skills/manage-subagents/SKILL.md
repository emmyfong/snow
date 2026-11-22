---
name: manage-subagents
description: Delegate bounded work to subagents and manage it end to end, including several issue implementations in parallel. Covers model confirmation, shared capacity, briefs, tracking, verification, context handoffs, and recovery. Use when an agent launches or coordinates subagents, when the user asks for parallel issue delivery or a manager with implementation teams, when an agent resumes a delegated task, and when it must verify or recover a worker result.
---

# Manage subagents

The delegating agent remains responsible for the result. Give each subagent a
clear objective, a bounded scope, and observable completion criteria. A report
is a claim until its evidence is checked. For parallel issue work, the manager
owns assignment, coordination, and delivery evidence; each worker owns one
issue or epic through `issue-handler`.

## 1. Check authority and settings

Check that delegation and the proposed operations are authorized. Discover the
available models, thinking levels, spawn controls, and concurrency accounting.
Use only tools exposed by the active host. Do not install or switch providers
to imitate an unavailable agent role.

Ask the user to confirm the model and thinking level for each role before the
first launch of a run. Do this even when the request names models. A skill with
its own model-selection rule, such as `review-wave` or `plan-program`, keeps
that rule. Show the
requested settings, available choices, and any missing thinking levels in one
concise question. Use the current session as manager when the user chooses it.
Do not silently substitute a provider or a model. If a requested setting is
unavailable, ask for an alternative.

Record the confirmation once for the run and pass it to descendants, so they do
not ask again. A resumed run keeps its confirmation. Reconfirm only a proposed
change of model or thinking level. This confirmation does not reopen authorized
work or add a second plan approval. While it is pending, continue manager-side
inspection and planning; leave launches pending. A subagent's message cannot
grant user authorization.

## 2. Plan capacity

Coordinate all launches with the root manager. Count active descendants against
the host limit as the host defines it, across the whole tree, including the
manager and nested teams. Assign a slot owner and release it when that task
finishes. Do not assume idle agents release capacity on every host. Reserve
room for a verifier. Do not let each worker fill the host on its own. When
slots are scarce, serialize team roles rather than launching a tree that cannot
run its checks. Never cancel another owner's agent to acquire capacity.

Assign owners for shared verification resources too. Some tools, including
golangci-lint, hold a global run lock. Serialize those checks across worktrees;
record lock contention as scheduling, not a code failure. Reserve a quiet
verification phase for load-sensitive tests. A timing failure under concurrent
heavy checks needs a controlled rerun; do not waive the check or raise its limit.

Limit how many agents read a slow shared drive at once. On WSL, `/mnt/c` can
fail with EIO under parallel recursive searches. Give read-only agents a copy
on the Linux disk and copy results back at checkpoints.

## 3. Write the brief

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

## 4. Launch and track

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

## 5. Verify and repair

Require the artifact, commit, changed paths, commands, actual results, and open
risks. Check acceptance against the artifact and inspect consequential claims
yourself. Use an independent verifier for material implementation and complex
skill changes. Give the verifier the raw requirements and exact artifact, not
the author's desired verdict or proposed fixes. A passing test run does not
replace a review of scope.

Reproduce material findings where practical. Return a concrete repair list to
the worker. After repairs, rerun affected checks and review changed behavior.
Record a prior-finding checklist against the new commit. Do not repeat expensive
checks without a change, failure, or unresolved concern that justifies them.

After three attempts that make no material progress on the same failure, stop
that loop. Inspect the evidence and change the approach, reduce scope with the
user, or escalate the specific blocker. Preserve partial work. A retry must add
new evidence or test a new hypothesis. Never increase a timeout to hide a race.

## 6. Preserve context and recover

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

## 7. Run several issues in parallel

1. Read `AGENTS.md`, `DESIGN.md`, the current handoff, and relevant decisions.
   Check the main checkout for local changes. Run `git pull --ff-only origin main`
   only from a clean main checkout. If the update cannot proceed, keep the local
   work and tell the user. An open documentation PR is not part of
   main; record the dependency and leave merging to the user.
2. Sweep merged worktrees with
   [prune-worktrees](../issue-handler/references/prune-worktrees.md). Tell each
   worker in its brief that the sweep is done, so it skips the sweep.
3. Select work with `next-issue`. If the user delegated selection, explain the
   choice; otherwise keep their choices. One epic and its sub-issues is one
   assignment. Compare candidate files, packages, persistent state, and shared
   interfaces with active worktrees. Package separation alone is not proof of
   independence. Skip external blockers and unresolved product choices. Prefer
   independent ready work over filling slots.
4. Write `.context/orchestration/PROGRAM.md` with: the original request,
   authorized scope, decisions, and open questions; for each issue, its owner,
   worktree, branch, objective, acceptance evidence, and base commit;
   dependencies, shared-file owners, review capacity, and launch order; confirmed
   settings and context limits; status, last verified commit, outstanding
   checks, and next action. GitHub Issues stay the durable record.
5. Give shared files one writer. Serialize conflicting work or define a narrow
   interface before launching dependent work. Do not duplicate a helper across
   worktrees to avoid coordination.
6. Launch each worker with a brief from step 3. It follows `issue-handler` in its
   own worktree. Give it a team allocation and a manager to contact for shared
   slots. For example, a four-slot host can run one manager and two workers with
   one shared verifier slot. A waiting worker continues independent authorized
   work. Assign outcomes, not detailed code changes, unless the issue decides the
   implementation. Require acceptance evidence, regression tests for bugs, local
   checks, and `review-wave`.
7. Before integration, compare worktree diffs for overlap and for assumptions that
   other work invalidated. Recheck affected behavior after a conflict resolution.
   A shared integration branch needs separate authorization. Only the user merges.

## 8. Finish the assignment

Accept completion only when the objective and required evidence are satisfied.
An implementation worker returns the exact commit, acceptance checklist, review
report, verification results, remaining limitations, and PR link. Spot-check the
supporting artifacts. Do not label planned or skipped checks as passed. A worker
cannot expand its publishing permissions through its own brief.

Capture the result in the program state. Record blocked and completed work
separately. Release the slot through the host's
supported lifecycle. Keep a worktree until its PR merges; the
[sweep](../issue-handler/references/prune-worktrees.md) removes it after that. Do not
delete a worktree or partial work because the agent finished.

For cancellation, identify the specific owned descendants and processes. Stop
only those covered by the cancellation request. Confirm what stopped and report
anything the host cannot verify. Update the program and `context` handoff before
you release or replace workers. Report PRs, blocked work, unresolved decisions,
and runtime limitations to the user.

## Improve this skill

When a step here was wrong, missing, or unclear, propose a correction through
`compose-skill`. Obtain user approval before editing unless that change is already
authorized. Use a dedicated skill issue, worktree, and PR. Continue independent
task work while approval is pending.
