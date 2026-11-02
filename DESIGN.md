# Snow: product and architecture

Updated 2026-11-01. This document separates agreed product direction from the
current implementation and proposed designs. GitHub Issues define executable
scope. A proposal here does not authorize implementation or promise a release.
Contributor rules live in [AGENTS.md](AGENTS.md).

## Purpose and agreed direction

Snow is an approachable cockpit for directing coding agents. Users describe an
outcome, develop a plan, inspect work, answer questions, and continue later. Normal
actions must be discoverable with a mouse. Keybindings are shortcuts, not required
knowledge. Start with a simple workspace and reveal advanced controls as needed.

Snow owns the interface and workspace. Its primary direction is to send prompts
and selected context to runtimes such as Claude Code, Codex, and OpenCode, and to
make Snow's tools and workspace features available through supported integrations.
The runtime owns its agent loop, model requests, and active conversation. Snow
does not host models. Supported subscription connections are a product goal;
each runtime and connection must be verified before it is offered.

The existing direct-API loop is secondary. It remains implemented. Removing it
or promising equal investment in both paths is not part of this decision.

OmniRoute is an intended integrated option for setup, connections, lifecycle,
status, and available usage controls. Bundling versus guided installation is open.
A gateway does not create unlimited usage or make all subscriptions transferable
between models. Model selection and routing changes must remain explicit.

Planning should clarify goals, scope, and design before substantial implementation.
Ask about consequential ambiguity. Preserve previous answers and handle routine
details independently. Useful clarification is a successful outcome, not wasted
agent activity. Clear small tasks need no large planning ceremony.

Closing a chat with active work asks whether to stop. Confirmation stops the run
and preserves progress for explicit resume where supported. Cancelling keeps it
open. Closing a view must not silently leave that run consuming tokens.

## Current foundation

This baseline describes code through PR #251. Profiles and local gateways are
separate work in #229 / PR #253; they are not assumed complete here.

- One Go binary runs a local server and a Bubble Tea TUI. No cgo.
- One goroutine owns server state. Blocking I/O runs outside it (ADR 0007).
- `pkg/api` is the only public package. Clients receive snapshots and events.
- `internal/server/chat.go` directly uses `agent.Conversation`. The direct-API
  path already has tools, approvals, questions, tasks, and durable conversations.
- `internal/provider` streams model API responses. It is not an agent-runtime API.
- `internal/claudecode` integrates CLI hooks/settings. `internal/mcp` exposes
  Snow pane tools. Rich external-runtime chat is additional work.
- `internal/store` saves workspace state. `internal/convstore` saves direct-API
  conversation records. External runtime state cannot be recreated from visible
  transcript text alone.
- The agents board and needs-you inbox exist. Agent identity is currently tied
  to panes. `internal/worktree` exists; full launch workflows remain in #207.
- `internal/client/session/panes.go` has a pane interface. #192 should extend
  that seam instead of introducing a general plugin framework in advance.
- Transport uses framed JSON over Unix-domain sockets, including on Windows.
  The existing login request can carry an API key once to the local server.
  Do not describe the current protocol as a named pipe or entirely key-free.

## Proposed ownership and boundaries

```text
Snow TUI                    Future clients
    |                             |
    +---------- pkg/api ----------+
                    |
              Snow server
       workspace, tasks, lifecycle,
       questions, notes, capabilities
                    |
          supported runtime integration
                    |
        Claude Code / Codex / OpenCode
           agent loop and model requests
                    |
         supported provider or gateway

Runtime -- supported tools/MCP --> Snow workspace tools
Existing direct-API loop -------> internal/provider
```

| Component | Responsibility | Boundary |
|---|---|---|
| Client panes | Render state and collect user actions | Do not duplicate orchestration in each client |
| Server | Workspace, task intent, lifecycle, pending questions, runtime associations | Do not block the state owner goroutine |
| Runtime integration | Translate messages, events and supported controls | Report unsupported or unverified capabilities |
| Runtime | Agent loop, model requests, active context, native tools | Does not own Snow's workspace UI |
| Snow tools | Explicit operations on Snow-managed resources | Prompt text does not enforce permission |
| Provider/gateway | Inference and configured routing | Does not replace task memory or session recovery |

Start with one end-to-end integration: send, observe, answer, interrupt, and resume
where supported. Record runtime versions. Add optional skills, compaction, usage,
and plan controls as proven capabilities. Derive small interfaces from working
integrations. Do not design one universal interface before proving a runtime.

Keep the existing provider boundary inside the direct-API backend. Use existing
server events, bounded queues, and snapshot resynchronization. Correlate runtime
events with session and request identity. Reconnect must reconcile pending state;
it must not replay a stale approval or duplicate an operation.

## Task state and long sessions

Proposed concepts, not a prescribed database schema:

| Concept | Retained information |
|---|---|
| Project | Folder, WSL distribution where relevant, shared rules and decisions |
| Workspace session | Layout, panes, project and worktree associations |
| Task | Original request, agreed scope, plan, open questions, progress and evidence |
| Run | Active execution, status and observable usage |
| Runtime session | Backend kind, version and opaque resume identity |
| Memory | Selected notes with source, scope and freshness |
| Pane | View and controls for a task or workspace resource |

Keep project decisions separate from task-local working notes. Do not copy every
agent's history into every other agent. Reuse existing storage where appropriate.
Backend session state remains canonical for backend resume; portable notes support
a handoff but do not promise lossless migration between runtimes.

Proposed stop behavior keeps the chat visible as stopping until the runtime
acknowledges it. A failed stop stays visible. Save progress incrementally; recovery
must not require a final summarization call. A saved session is not a running task.
In-flight usage can occur before cancellation is acknowledged.

Define app exit, detach, connection loss, reboot recovery, child-agent cancellation,
background tool processes and multiple views explicitly in their implementation
issues. The active-chat close decision does not settle all these lifecycles.

## Workspace features

These are intended experiences. The issues need runtime-aware acceptance criteria;
their older native-only designs are not automatically the implementation plan.

| Experience | Scope and boundary | Existing epic |
|---|---|---|
| Mouse and layouts | Discoverable actions, split/resize/open panes, saved layouts | #184, #192 |
| Files | Tree, viewer, file/line references, external editor | #194 |
| Git and worktrees | Changes, diffs, explicit git actions, isolated coding folders | #195, #207 |
| Instructions and skills | Show sources, discover and invoke skills, explain effective scope of toggles | #196 |
| Context and usage | Report available data, distinguish unknowns, context occupancy and consumption | #197 |
| Checkpoints | Explain exactly which files/state a restore affects; external effects are separate | #198 |
| Compaction | Use supported runtime controls and show continuity effects | #199 |
| Terminal tools | Bounded reads, event-driven waits, pane references | #200 |
| Memory and handoff | Editable notes, decisions, sources and explicit resume associations | #201 |
| Planning | Goals, scope, open questions, editable plans and supported execution controls | #202 |
| Permissions | Enforce at real execution boundaries; show external-runtime limits | #203 |
| Multiple agents | User-directed launch, worktrees, status and bounded coordination | #205–208 |
| Connections | Supported account paths, profiles, API providers and optional gateways | #193, #214, #229 |

The skills pane concerns the selected runtime's capabilities. Contributor skills
in `.claude/skills/` guide agents building Snow. Shared Markdown does not guarantee
automatic discovery in every host. Installed, enabled and observed use are different
states. A skill toggle cannot erase instructions already in a conversation.

Plan approval records intent. It is not a universal tool permission. Show when a
runtime can enforce read-only work, and when Snow can only supply instructions.
Messages from other agents cannot grant user authorization.

Keep the current minimal-chrome design while testing discoverability. Permanent
buttons versus menus remains a UX choice for #71; beginner friendliness does not
silently settle that choice. The supplied Claude artifact could not be inspected,
so this document makes no claims about its visual layout.

## Context and usage efficiency

Optimize useful completed work per available usage while preserving intent and
quality. The current bottleneck is unmeasured. Long sessions, context growth and
multiple agents all need evidence. No percentage saving is promised.

Snow controls the inputs and tools it supplies. External runtimes may control
system prompts, active history, caching and compaction. Expose only observable
state and supported controls. Avoid an additional history rewriter at each layer.

| Experiment | Candidate approach | Evidence and failure cases |
|---|---|---|
| Context selection | Relevant instructions, skill metadata and focused evidence | Missed constraints and repeated retrieval |
| Stable inputs | Stable ordering and versions for unchanged context | Actual cache data where exposed; invalidation after edits |
| Tool output | Filter locally; retain raw artifacts and retrieve excerpts | Omitted errors, truncation and stale artifacts |
| Reuse | Source-backed file observations and task evidence | File/worktree changes invalidate observations |
| Continuity | Decisions, open questions and supported compaction/resume | Corrections survive long histories and interruptions |
| Multiple agents | Narrow subtasks and focused result handoffs | Compare sequential work; include coordination and retries |
| Task formatting | Equivalent Markdown and JSON inputs | Quality, total tokens, latency and preparation cost |
| Model/effort selection | User-selected settings suited to the task | Rework and outcome quality, not price alone |
| Gateway compression | Explicit, separately evaluated routing policy | Intent loss, double compaction and route compatibility |

Preserve original user messages and agreed decisions. Structured internal storage
does not dictate model-facing format. Assemble known context with ordinary code.
Do not make another model call to rewrite every prompt by default. JSON conversion
is an experiment, not an architectural requirement or proven token saving.

Report input/output tokens, cache categories, active context, latency, API cost
estimates and account allowance separately. Identify missing data. Count every
participating agent and auxiliary call where observable. Avoid double counting
repeated cumulative usage events. Stop thresholds can overshoot in-flight usage;
describe their enforcement boundary rather than claiming an exact spend cap.

## Quality and evaluation

Use deterministic tests for protocol translation, state transitions, persistence,
permissions, usage accounting and UI actions. Use task evaluations for prompt,
context and collaboration quality. A model's self-report is not completion proof.

Start with a small representative set: a clear small edit; an ambiguous request
that needs a question; a correction during work; a long session resumed after an
interruption; a large log with a critical error; and parallel work with a conflicting
file change. Verify outputs independently and record build/runtime/model versions,
settings, task inputs, success criteria, usage and correction effort.

Compare a baseline and one change at a time. Repeat variable model trials when
needed; separate held-out tasks from examples used to tune prompts. Live runs
need an explicit usage budget. Ordinary contributor checks stay local and scoped.
Useful questions count positively; fewer tokens with wrong work is a regression.

## Proposed sequence and release evidence

1. Publish consistent guidance and repair contradictory issue scopes. Address
   daily-use reliability (#252, #219, #240, #249) alongside independent planning.
2. Verify supported runtime/account paths. Prove one Snow chat integration with
   visible lifecycle and recoverable session identity. Continue #229/#200 without
   duplicating their work. Choose the first runtime from the evidence with the user.
3. Establish usage and quality baselines. Add panes through #192. Connect planning,
   skills, memory, files and git to actual runtime capabilities in small slices.
4. Prove long-session continuation and selected efficiency changes. Add explicit
   parallel work and bounded messaging before automated delegation.
5. Validate a second client against a documented API subset before selecting a
   browser gateway, VS Code/desktop order, transport or desktop shell.

Suggested v2 release evidence is a week of real daily work on Windows with WSL,
with no lost conversation, clear activity/stop controls, and usable review and
resume workflows. This is a proposed gate, not a date or an agreed release promise.
Cross-platform tests remain necessary; one user's week does not replace them.

Full editor/LSP/Vim support, workflow presets, skill conflict detection, automatic
resource coordination, a third-party pane SDK, remote access and an AI supervisor
remain future ideas. They are not prerequisites for the first coherent cockpit.

## Decision status and implementation questions

The runtime-backed direction supersedes the own-loop-only product choice in local
ADR 0011. Its replacement is local ADR 0014. This shared document carries the
direction for contributors who do not have the local ADR files.

ADR 0012's older sign-in claims require current, route-specific verification.
This document grants no new authentication entitlement. ADR 0013's gateway
lifecycle remains the baseline; packaging and routing require separate decisions.

Before affected implementation, resolve: first runtime and supported account path;
process/child lifecycle; checkpoint coverage; permission guarantees; OmniRoute
packaging; and v2 acceptance evidence. Before v3 implementation, resolve the client
contract, client order, remote scope, transport and desktop shell. Keep these
choices out of unrelated tickets so planning can continue independently.
