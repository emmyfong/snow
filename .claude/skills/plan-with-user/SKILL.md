---
name: plan-with-user
description: Clarify a product idea or ambiguous feature with the user and develop a scoped design before implementation. Use for collaborative discovery and planning; not for routine execution of an already clear task.
---

# Plan with the user

Produce a shared understanding of the outcome, scope, and design. Match planning
depth to uncertainty. A request to explore is not an instruction to implement.

## Establish intent

Read relevant project guidance, existing issues, and accepted decisions. Reuse
answers already supplied. Distinguish the user's requirements from your proposals
and assumptions. Explain contradictions that affect the plan.

For architecture work, read the relevant sections of DESIGN.md. Distinguish
existing behavior from intended product behavior and untested proposals. Identify
the component that can enforce each promise, especially for external runtimes.

Frame the user problem and a concrete example of success. Ask about the ambiguity
that most affects the next decision. Explain the consequence of the choice and
offer a recommendation when evidence supports one. Do not ask the user to supply
facts available in the project, or force a fixed questionnaire on every task.

## Develop the smallest adequate design

Establish scope, non-goals, and constraints before choosing components. Search for
existing capabilities. Compare a simpler approach with broader alternatives where
that tradeoff matters; justify new packages, services, dependencies, or workflows.
Do not optimize for a line count or remove required behavior to make code smaller.

Ask before relying on assumptions that materially alter behavior, scope, cost,
data persistence, permissions, compatibility, or a substantial design commitment.
Handle routine details independently. Continue independent research while waiting;
do not begin work that depends on an unanswered consequential choice.

Keep the user informed when findings change the recommendation. If the user changes
direction, update the plan and retire conflicting assumptions. Do not treat silence,
an agent's summary, or a draft document as agreement.

## Leave a usable brief

Record the problem, success example, scope, non-goals, constraints, proposed or
agreed design, alternatives, unresolved choices, and validation approach. Use only
the detail needed for another agent to act without inventing product requirements.
Label proposals and decisions separately. Point to their sources rather than
duplicating long transcripts or maintaining competing plans.

Resolve implementation-blocking product choices before handing off. Preserve
previous authorization; do not add repeated approval gates. For Snow, turn agreed
work into existing or new GitHub issues through the repository's issue workflow.
Do not start implementation or publish changes merely because planning is complete.

## If implementation reveals new uncertainty

Return to the affected decision when evidence invalidates the design or materially
changes scope. Explain what changed and the smallest viable alternatives. Do not
silently expand the assignment, restart all planning, or interrupt for routine details.
