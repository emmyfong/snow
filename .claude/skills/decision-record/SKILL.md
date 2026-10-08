---
name: decision-record
description: Record a settled design decision as an ADR in docs/decisions/, or supersede an old one. Use when the user and agent settle a design choice (stack, naming, interface shape, policy), or before an agent reopens or contradicts a decision that may already be recorded.
---

# Record design decisions as ADRs

An Architecture Decision Record (ADR) states one decision, why it was made,
and what it costs. ADRs stop agents from reopening settled questions.

## Before you reopen a decision

Before you propose a change to a decided topic, search the records:

```sh
grep -ril '<topic words>' docs/decisions/
```

If an ADR covers it, follow it. If you think it is wrong, tell the user and cite
the ADR. Change it only after the user agrees, by superseding it.

## Write a new ADR

1. Confirm the decision with the user in one sentence. Do not record a
   decision the user has not made.
2. Find the next number:
   ```sh
   ls docs/decisions/ 2>/dev/null | grep -oE '^[0-9]{4}' | sort -n | tail -1
   ```
   Add 1 and pad to four digits. The first ADR is `0001`.
3. Name the file `docs/decisions/NNNN-<kebab-case-slug>.md`. Create the folder
   if it is missing.
4. Fill `templates/adr.md`. Keep it under one page. Write in ASD-STE100 style.
5. Show the user the path.

## Supersede an ADR

1. Write the new ADR. In its Context, name the old ADR and say what changed.
2. Edit the old ADR's Status line to `Superseded by NNNN`. Change nothing else
   in the old ADR.

## docs/ is local

`docs/` is gitignored, so ADRs exist only on this machine and are not shared
through git. If every agent must follow a rule from the decision, also add
the rule to `AGENTS.md`, which is committed. Ask the user first.
