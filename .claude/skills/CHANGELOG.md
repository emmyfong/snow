# Skill changelog

Agents add an entry at the top for every skill change (see `compose-skill`).
Approved skill changes use a dedicated issue, worktree, and PR. The user reviews
`pending review` entries. Historical entries record the policy at their date;
they do not override the current approval rules.

## 2026-11-27 review-wave: run a full host test before PRs
- Evidence: PR #507 changed shared theme tokens. Package-scoped checks missed
  Files and Home golden failures on Ubuntu, macOS, and Windows.
- Change: run one full host-platform test suite on the final tree before PR.
  Keep package-scoped checks during implementation. Reuse an unchanged result.
- Test: scenario walkthrough: a theme token changes while its Files and Home
  consumers are untouched. The final gate runs all packages and catches stale
  consumer goldens before PR. The `AGENTS.md` rule and local-gate reference
  agree on commands and reuse conditions.
- Status: pending review

## 2026-11-25 tui: catalog ButtonRow and pane-local messages
- Evidence: #488 identifies the missing ButtonRow catalog entry from #481 and
  the StatusBar wording that conflicts with #483's pane-local messages. The
  user approved both exact rows on 2026-11-25 before editing. At base
  `c407660`, `internal/client/ui/buttonrow.go` does not exist. The signatures
  come from #481, not from an implementation on main.
- Change: limit the StatusBar message rule to session messages and notices.
  Describe messages in a pane's own status row. Add the ButtonRow signatures,
  width, label style, drop order, right-edge label, click spans, and ChoiceBox
  one-row use with its multi-row fallback. Change no other catalog rows.
- Test: scenario: a contributor adds Stage and Diff buttons with Commit at the
  right edge, then handles a refused stage in the git pane. Expected behavior:
  reuse ButtonRow and its spans, drop Diff first in a narrow row, keep Commit
  when it fits alone, and show the refusal in the pane's status row.
  Manual walk-through with AGENTS.md and the changed tui skill: the routing
  selects tui; reuse leads to ButtonRow; the row keeps its width and does not
  wrap; the same options supply click spans; the StatusBar row puts the
  refusal in the pane while session notices stay on the status line.
  ChoiceBox keeps one button per row when its buttons do not fit one row.
  The guidance uses file paths and Go signatures with no host-specific action,
  so the steps are readable in Claude Code, Codex, and OpenCode. This is a
  manual instruction walk-through, not an executed cross-host or runtime test.
  Whitespace, skill frontmatter, references, table structure, exact approved
  wording, and the two-file scope checks pass. Go and visual checks do not
  apply because no product code or rendered output changes.
- Status: pending review

## 2026-11-21 AGENTS.md, issue-handler, next-issue, manage-subagents: skill routing and merged-worktree sweep
- Evidence: on 2026-11-21 the main checkout had 26 worktrees; 21 had merged
  PRs. `issue-handler` step 8 removes a worktree only after the user merges,
  and the agent stops at step 7, so step 8 never ran. The extra files made
  parallel searches over `/mnt/c` fail with EIO three times. `AGENTS.md` listed
  the skills without telling agents to use them. `agent-orchestration` and
  `manage-subagents` repeated the same settings, capacity, and verification
  rules. `plan-with-user` repeated the `AGENTS.md` planning section. In Claude
  Code, `superpowers:brainstorming` ran instead of `plan-with-user`. The user
  asked for the cleanup rule, the routing, and both merges.
- Change: new `issue-handler/references/prune-worktrees.md`, called from
  `issue-handler` step 2, `next-issue` step 1, and `manage-subagents` step 7.
  `AGENTS.md` has a removal rule and a routing table for every skill, and its
  planning section is four rules plus a pointer to `plan-with-user`.
  `agent-orchestration` is merged into `manage-subagents` step 7 and removed.
  `plan-program` starts with `plan-with-user` when the goals are unclear.
  `CLAUDE.md` gives Snow skills precedence for planning. After review, the
  sweep checks only `.worktrees/`, skips the current worktree, keeps on any
  command error or open PR, and does not run `git worktree prune`. Only the
  top-level agent sweeps; a worker that a manager launched skips it.
  `issue-handler` step 8 runs the sweep for its own worktree.
- Test: three scenarios, walked through with only `AGENTS.md` and the skills.
  (1) Start an epic while a merged worktree exists: the routing table gives
  `issue-handler`, and its step 2 runs the sweep first. Through `next-issue`,
  step 1 runs it. (2) Run three issues in parallel: the table gives
  `manage-subagents` step 7, which sweeps, selects with `next-issue`, and
  launches `issue-handler` workers. (3) Plan one new idea: the table and
  `CLAUDE.md` give `plan-with-user`, then `compose-issues`. The walk-through
  found that the sweep used `<root>` before `issue-handler` defined it; the
  reference now defines it. The 2026-11-21 sweep of the main checkout removed
  21 merged worktrees and kept 5 (no PR or detached HEAD). One merged PR head
  was missing locally, so the reference now checks ancestry through GitHub.
  A review wave (two reviewers) on 5884ea8 found 4 major, 10 minor, and 1 nit
  findings; all are fixed. Documentation checks passed: whitespace, front
  matter, links, and no remaining `agent-orchestration` reference.
- Status: pending review

## 2026-11-17 tui: describe the theme background and bar colors
- Evidence: #318 and #319 change the theme defaults and bar colors. In the
  #316 terminal check, conhost applied the requested colors but ignored the
  reset when Snow exited.
- Change: describe the default color reports, the terminal option, and the
  shared bar colors and active-tab rules in the TUI catalog.
- Test: a contributor checks a pane with default and explicit text in each
  mode. The guidance prescribes applying defaults only after both reports and
  keeping explicit cell colors. In Windows Terminal through WSL, Snow
  restores the colors the shell set before launch. Native conhost keeps its
  defaults and draws the pane's explicit color. Reduced-color tests cover
  secondary text on the painted background. Skill frontmatter and
  whitespace checks pass.
- Status: pending review

## 2026-11-14 tui: describe layout B chrome
- Evidence: #294 records the user's 2026-11-10 layout B decision and identifies
  conflicting minimal-chrome guidance.
- Change: replace the old permanent-buttons rule and beginner guidance with
  layout B guidance. Update the shared design summary and local ADRs 0016 and
  0017 to carry the accepted chrome and theme decisions.
- Test: scenario: a contributor follows the TUI skill while implementing layout
  B. The rule identifies the window bar, status line, pane labels, focused-pane
  buttons, menu, and shortcut behavior; the beginner guidance points to visible
  discovery paths. Skill frontmatter validation, link inspection, `git diff
  --check`, and `go build ./...` passed.
- Status: pending review

## 2026-11-12 compose-skill: approve improvements before separate skill work
- Evidence: user revised the immediate-edit and same-task-PR policy after #354. Tracked in #359.
- Change: require a concrete proposal and approval before agent-initiated edits; honor explicit requests; use a dedicated issue, worktree, and PR. Align all current Improve this skill sections and remove the issue-handler exception.
- Test: fresh Luna walkthrough kept unapproved discoveries unchanged, honored explicit authorization, respected refusal, continued independent work, and separated routine review fixes from new permission scope. Clarified publication scope after the walkthrough; structural validation passed.
- Status: pending review

## 2026-11-12 agent-orchestration and manage-subagents: verified parallel delivery
- Evidence: user requested parallel Sol workers with Luna teams, explicit model and thinking confirmation, verification loops, and 40% compaction in #355.
- Change: add manager and subagent lifecycle workflows; reuse issue-handler, review-wave, context, and compose-skill. Document host capacity, shared lint-lock scheduling observed in #292, and compaction enforcement limits. Follow the user's revised approval-first, separate skill PR policy (#359).
- Test: fresh Luna scenario evaluation correctly handled model confirmation, four-slot allocation, conflicting/blocked issues, failed checks, stale review, unavailable compaction, and an uncertain writer process. Clarified pre-launch manager-side work after the walkthrough. Both structural validators and relative-link checks passed.
- Status: pending review


## 2026-11-12 compose-skill, issue-handler: a skill fix ships in the task's PR
- Evidence: the #353 review found that "edit in your task's worktree"
  conflicted with the batch review and with the issue-handler scope rule. The
  user chose on 2026-11-12: a skill fix ships inside the epic PR.
- Change: `compose-skill` "Where to edit" puts the fix in its own
  `docs(skills):` commit on the task branch and names it under "Skill changes"
  in the PR body. The batch review lists where each entry is. `issue-handler`
  allows this one exception to its scope rule. `AGENTS.md` says the same.
- Test: scenario: an issue-handler session finds a wrong `tui` step. Walk-through
  result: the agent fixes the step in its worktree, commits it apart from the
  feature commits, logs it here, and lists it in the PR body. No step sends it
  to another branch.
- Status: pending review

## 2026-11-12 compose-issues: prepare bodies and check links before publishing
- Evidence: publishing 77 bodies on 2026-11-11 found planning headers,
  revision notes, bare planning keys, and local file paths in the bodies. One
  `gh issue create` returned a GraphQL error after it created the issue.
- Change: step 6 strips headers, posts a `## Revision` section as one comment,
  resolves bare and existing-issue keys, and replaces local file references. It
  checks for a duplicate after a failed create, and checks the parent and the
  current blockers before linking. Step 7 compares each posted body with its
  prepared file exactly and counts native sub-issues.
- Test: scenario: publish a workstream with new and existing issues. Result:
  the 2026-11-11 run followed these steps and read back 77 bodies with no
  mismatch and no duplicate issue.
- Status: pending review

## 2026-11-12 plan-program: no probes, marked defaults, one way, safe publishing
- Evidence: in the 2026-11-10 run a drafter ran a Codex probe against the
  user's real `~/.codex`, and an OpenCode command tried to start a machine-wide
  service. Drafters chose user-visible defaults as if decided. The user found two
  key systems (Alt keys and a tmux preset) confusing and chose one.
- Change: drafters and verifiers run only read-only `git`, `grep`, and `gh`;
  implementer probes run in an isolated profile with the user's approval;
  drafter defaults are marked for user confirmation; one way to do a thing;
  fix lists as files; the manager reads pages a verifier cannot open; an
  approval page with numbered defaults; rework and verify again when an answer
  changes a used decision; publish with a manifest and a state file. The
  drafter and verifier briefs carry the same command list.
- Test: scenario: plan one workstream that depends on an installed CLI.
  Result of the walk-through: the drafter writes the probe as an issue step
  with a stop rule and runs nothing; the manager asks about each marked
  default before publishing.
- Status: pending review

## 2026-11-10 compose-skill, plan-program: add self-improving skills and program planning
- Evidence: the user asked for skills that update themselves, a skill
  composer, and a manager session that plans the whole project with drafting
  and verifying subagents, so that Codex and OpenCode can implement the epics
  without questions.
- Change:
  - New `compose-skill`, with `references/skill-format.md`.
  - New `plan-program`, with drafter and verifier briefs.
  - New `compose-issues/references/implementer-ready.md`. `compose-issues`
    step 4 now requires it and points to `plan-program`.
  - Every skill ends with "Improve this skill".
  - `AGENTS.md` lists the two skills and the self-improvement rule.
  - New Claude Code agents `planner` and `plan-verifier` pin Haiku 5.5 (the
    user chose Haiku drafters and verifiers on 2026-11-10).
- Test: walk-through of "plan the whole of Snow as implementer-ready epics"
  against `plan-program` and `compose-issues`; the first live run is the
  planning program of 2026-11-10.
- Status: pending review

## 2026-11-15: Layout component catalog and header geometry (#294)

- Evidence: #295 and #298 replace bottom tabs and CloseCell. The original geometry had no top border for top-edge panes. The user approved reusing lower dividers and reserving top-edge headers without covering content.
- Change: document WindowBar spans, quiet StatusBar, focused PaneButtons and runtime control entry points. Remove obsolete bottom-bar hit-target catalog entries. Record the accepted geometry in DESIGN.md and local ADR0017.
- Validation: skill frontmatter and whitespace pass. Final code/API comparison and independent review passed against the accepted #293 implementation contract. A narrow split-chat scenario confirmed shared hit targets, header/content separation, and runtime-specific controls. Earlier guidance review corrected a false claim that ADR0018 already existed; the final wording keeps it pending under #301.
## 2026-11-14: Assign selected issues before implementation (#364)

- Evidence: the user requested explicit ownership while coordinating #260 and #293; both issues had no GitHub assignee.
- Change: issue-handler requires verified assignment and limits execution to the selected scope. next-issue can assign authorized selected work before handoff. It preserves other owners, checks child ownership, and separates account assignment from agent writer coordination.
- Validation: structural validation and whitespace checks passed. An independent Luna medium walkthrough confirmed that recommendations cause no writes; a direct implementation request assigns and verifies the selected issue; another owner or a friend-owned child stops affected work; assignment failure leaves implementation pending; and agents sharing an account keep their separate scopes. The reviewer found no material policy defect. The requested outcomes matched all scenarios.
