# Drafter brief: <workstream name>

Phase: <structure | bodies>
Repository: <absolute path of the main checkout>, planning commit <sha>.
Write only inside: `.context/compose/program/<ws>/`. Read anything.

## Goal
<What this workstream must let a user do when its epics ship. Two to four lines.>

## User decisions that bind you
- <decision, date, source>

## Owned issues
<Issue numbers and titles this workstream owns. Revise them; do not duplicate them.>

## Read first
- `AGENTS.md`, `DESIGN.md` sections: <names>
- ADRs in `docs/decisions/`: <numbers>
- `.claude/skills/compose-issues/SKILL.md` and its `references/`
- `.context/compose/glossary.md`
- `.context/compose/program/issues.json` (all open issues)
- Code: <packages and files to start from>

## Neighbors
<Other workstreams and what they own, so you do not plan their work. Name the
arrows you expect between them.>

## Output (structure phase)
`STRUCTURE.md` with:
1. Epics: title, labels, one-paragraph why, and owned or new sub-issues.
2. Each sub-issue: title, labels, one line of scope, blockers.
3. The DAG as an ASCII diagram, with arrows to other workstreams marked.
4. Re-homed, retitled, merged, or closed existing issues, each with a reason.
5. Open product questions, each with options, the tradeoff, and your
   recommendation.
6. Facts you could not verify, marked "unverified".

## Output (bodies phase)
One file per epic and sub-issue, named `<key>.md`, using `NEW-<key>` for new
issue numbers. Follow `body-sections.md` and `implementer-ready.md` exactly.

## Rules
- Read the code before you cite it. Every path and symbol must exist at the
  planning commit, or be marked "new".
- ASD-STE100 style. No em-dashes. Terms from the glossary or the code; add a
  new term to the end of your file under "Glossary additions".
- Do not publish, edit GitHub, commit, or change files outside your folder.
- Run only read-only commands: `git`, `grep`, `gh issue view`, `gh issue
  list`, and `gh api` GET. No probes, logins, or servers of the tools you plan
  for. Write a needed probe as an issue step.
- A default you choose for user-visible behavior is not decided. Mark it
  "Recommendation, user confirms at bodies approval".
- Finish with a short report: files written, open questions, unverified facts.
