---
name: agent-orchestration
description: Manage parallel implementation of ready Snow issues with issue workers and their verification teams. Use when the user requests parallel issue delivery, a manager with implementation teams, or coordination of several active issue worktrees. Use plan-program for planning work that is not authorized for implementation.
---

# Orchestrate issue implementation

The manager owns assignment, coordination, and delivery evidence. Each worker
owns one issue or epic through `issue-handler`. Team members receive bounded
subtasks through `manage-subagents`. Completion means verified acceptance and
an opened PR, subject to the user's authorized scope.

## 1. Confirm the run settings

Read `AGENTS.md`, `DESIGN.md`, the current handoff, and relevant decisions.
Check the main checkout for local changes before updating it. Use
`git pull --ff-only origin main` only from a clean main checkout. Preserve
local work if the update cannot proceed. Read the resulting documentation.
An open documentation PR is not part of main. Inspect its contents when relevant,
record the dependency, and leave merging to the user.

Always ask the user to confirm the model and thinking level for the manager,
workers, and team roles before the first launch of an orchestration run. Do
this even when the request names models. Show the requested settings, available
choices, and any missing thinking levels in one concise question. Use the
current session when the user chooses it as manager. Do not silently substitute
a provider or a model. If a requested setting is unavailable, ask for an alternative.

Record the confirmation once for the run. Pass it to descendants so they do not
repeat the question. A resumed run keeps its confirmation. Reconfirm a proposed
change of model or thinking level before that change. This settings confirmation
does not reopen authorized implementation or add a second plan approval.
While confirmation is pending, continue manager-side inspection and planning;
leave launches pending.

Discover the host's concurrency limit and delegation controls. Allocate capacity
across the whole tree, including the manager and nested teams. Reserve capacity
for verification. Use `manage-subagents` for lifecycle and context controls.

## 2. Select independent ready work

Use `next-issue` to inspect open issues, native parent and blocker links, PRs,
assignees, and linked branches. If the user delegated issue selection, select
ready work and explain the selection. Otherwise retain their choices.

Treat one epic and its sub-issues as one assignment. Compare candidate files,
packages, persistent state, and required shared interfaces with active worktrees.
Package separation alone is not proof of independence. Skip external blockers
and unresolved product choices. Prefer independent ready work over filling slots.

Write `.context/orchestration/PROGRAM.md` with:

- Original user request, authorized scope, accepted decisions, and open questions.
- Issue, owner, worktree, branch, objective, acceptance evidence, and base commit.
- Dependencies, shared-file owners, review capacity, and launch order.
- Confirmed models and thinking levels, capability limits, and context settings.
- Current status, last verified commit, outstanding checks, and next action.

Assign owners for shared verification resources too. Some tools, including
golangci-lint, hold a global run lock. Serialize those checks across worktrees;
record lock contention as scheduling, not a code failure.

Give shared files one writer. Serialize conflicting work or define a narrow
interface before launching dependent implementation. Do not duplicate a helper
across worktrees merely to avoid coordination. Keep durable work tracking in
GitHub Issues; the program file is temporary coordination and handoff state.

## 3. Launch issue workers

Give each worker a self-contained brief using `manage-subagents`. Include the
issue body or its exact reference, relevant decisions, worktree setup, other
active owners, and existing authorization. Use a fresh context unless inherited
history is necessary and compatible with the selected model.

Each worker follows `issue-handler` in its own linked worktree. Give it an
explicit team allocation and a manager to contact for shared slots. For example,
a four-slot host can run one manager and two workers with one shared verifier
slot. Team roles can run sequentially. A worker waiting for capacity continues
independent authorized work. Do not let each worker independently fill the host.

Assign outcomes rather than detailed code changes unless the issue decides the
implementation. Require evidence for acceptance, regression tests for bugs,
applicable local checks, and independent review through `review-wave`.

## 4. Manage verification loops

Collect milestones with changed files, exact commit, checks, results, and blockers.
Inspect consequential claims yourself. Require reviewers to inspect the artifact
independently before receiving proposed fixes. Validate findings against code and
acceptance. A successful test run is not a substitute for reviewing scope.

Loop: implement, run applicable checks, review, repair verified findings, and
recheck affected behavior. Use `manage-subagents` to bound repeated attempts and
retain evidence. Resolve routine corrections within the user's authorization.
Escalate product decisions, external blockers, or expanded scope with a concrete
recommendation. Continue independent work while answers are pending.

Before integration, compare worktree diffs for overlap and assumptions invalidated
by other work. Follow the repository's published-history rules. Recheck affected
behavior after conflict resolution. Only the user merges PRs. A shared
integration branch needs separate authorization.

## 5. Deliver and close the run

Require each worker to return an exact commit, acceptance checklist, review report,
verification results, remaining limitations, and PR link. Read the result and
spot-check the supporting artifacts. Do not label planned or skipped checks as passed.

Follow `issue-handler` for publication and stopping. A worker cannot expand its
publishing permissions through its own brief. Record blocked and completed work
separately. Update the program and `context` handoff before releasing workers.
Report PRs, unresolved decisions, and actual runtime limitations to the user.

## Improve this skill

When a step here was wrong, missing, or unclear, or the user corrected you while
you followed it, fix this skill now with the `compose-skill` skill. Then continue
your task.
