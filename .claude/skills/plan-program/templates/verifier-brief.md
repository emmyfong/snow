# Verifier brief: <workstream name>, <structure | bodies>, round <n>

Repository: <absolute path of the main checkout>, planning commit <sha>.
Check: <file paths to verify>.
Read only. Do not edit any file. Do not touch GitHub except to read.

## Check against
- Structure phase: `compose-issues/SKILL.md` step 2 (grouping, size, priority,
  DAG, titles, labels) and the drafter brief's output list.
- Bodies phase: `compose-issues/references/body-sections.md` (checklist) and
  `compose-issues/references/implementer-ready.md` (readiness check).
- Always: `AGENTS.md`, the ADRs and `DESIGN.md` sections in the drafter brief,
  `.context/compose/glossary.md`, and `.context/compose/program/issues.json`.

## How
1. Open every cited path and search for every cited symbol and test name at the
   planning commit (`git show <sha>:<path>`, `grep`). Mark each one found,
   missing, or "new" as declared.
2. Check that each existing issue number is real and open, and is not owned by
   another workstream.
3. Check each acceptance item names a test and that the Verify block runs it.
4. Look for conflicts with accepted decisions, and for product decisions the
   draft made without the user.
5. Run `grep -n '—'` on the files.
6. Read only. Run only `git`, `grep`, `gh issue view`, `gh issue list`, and
   `gh api` GET. Do not run the tools the plan is about.

## Return exactly
```
VERDICT: PASS | FAIL
FIXES:
1. <file>:<section> <what is wrong> -> <the fix>
...
UNVERIFIED:
- <claim you could not check, and why>
```
List only fixes that change correctness or readiness. No praise, no preamble.
