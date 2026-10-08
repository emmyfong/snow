---
name: compose-issues
description: Turn a set of planned or deferred work items into GitHub epics and prioritized sub-issues with a blocker DAG, get the user's approval on the structure and then on the bodies, and post them with native sub-issue and blocked-by links and existing labels. Use when the user wants to plan work as issues, break a roadmap or spec into epics, or file follow-ups from a review.
---

# Compose epics and issues

Turn loose items into vertical epics with prioritized sub-issues and a blocker
DAG. Two approval gates come before anything is created: the structure, then
the bodies.

## Preconditions

- `gh auth status` succeeds. `R=$(gh repo view --json nameWithOwner --jq .nameWithOwner)`.
- The user has named the items: a spec, a roadmap, review findings, or a list.

## 1. Inspect

- Collect every candidate item first. For each, record: the source, one line
  of substance, and whether an issue already exists.
- Search for existing epics and duplicates:
  ```sh
  gh issue list -R "$R" --label epic --state open --limit 50 --json number,title
  gh issue list -R "$R" --state open --limit 200 --search '<terms> in:title,body' --json number,title
  ```
- Read the code and ADRs the items touch, so bodies name real paths.
- Never create an epic that exists. Never duplicate an open issue. Re-home the
  item into the existing epic instead.

## 2. Draft the structure

- **Group vertically.** An epic is one concern a user can name, for example
  "Shell in a pane on every OS". Do not group by package or by layer.
- **Size issues for one PR.** One issue is one branch and one PR. Split an
  item that would touch many packages at once.
- **Prioritize.** P0 blocks users now. P1 is next. P2 is planned. P3 is when
  convenient. An epic's priority equals its highest child. Give one sentence
  of reason for each call that is not obvious.
- **Build the DAG.** For each pair of items ask: must one finish first for the
  other to be correct or cheap? Typical arrows: a decision or spike before the
  work it scopes; an interface before its implementations; a shared helper
  before its callers. Say which items are independent.
- **Titles.** Epics: `[P1][Epic] <title>`. Issues: `[P2] <title>`.
- **Labels.** Pick one type label and at most one component label per issue
  from `gh label list -R "$R"`.

## 3. Approve the structure

Present one message: the epics, every issue with priority and labels, the DAG
as an ASCII diagram, and any re-homed or retitled items. End the turn. In the
next turn, ask for approval with AskUserQuestion. Apply corrections and
present again when a grouping, priority, or arrow changes. Write no bodies
before approval.

## 4. Draft the bodies

- Use the sections in `references/body-sections.md`.
- Write each body to `.context/compose/<key>.md`. Use `NEW-<key>` for issue
  numbers that do not exist yet.
- Keep one glossary in `.context/compose/glossary.md` so all bodies use the
  same terms.
- With more than six bodies, you may delegate drafting to subagents. Ask the
  user which model to use. Give each subagent the glossary, the section
  rules, and one body to write.
- Review every body against the checklist in `references/body-sections.md`.

## 5. Approve the bodies

Give the user the folder path. Show epic bodies inline. Ask for approval with
AskUserQuestion. Apply corrections. Create nothing before approval. If you find
an ambiguity after approval, ask again before you change a body.

## 6. Create

Order: issues, then placeholder substitution, then epics, then links, then
labels.

Bodies go only through `--body-file` and titles only in single quotes. A
double-quoted body loses backticks and `$` text.

```sh
gh issue create -R "$R" --title '[P2] <title>' --body-file .context/compose/<key>.md
```

After all issues exist, replace every `NEW-<key>` with `#<n>` in all drafts.
Run `grep -rn 'NEW-' .context/compose/`. It must print nothing. Re-post the
bodies that had placeholders:

```sh
gh issue edit <n> -R "$R" --body-file .context/compose/<key>.md
```

Then create the epics with `--label epic`. Then link:

```sh
CHILD_ID=$(gh api "repos/$R/issues/<child>" --jq .id)
gh api -X POST "repos/$R/issues/<epic>/sub_issues" -F sub_issue_id="$CHILD_ID"
BLOCKER_ID=$(gh api "repos/$R/issues/<earlier>" --jq .id)
gh api -X POST "repos/$R/issues/<later>/dependencies/blocked_by" -F issue_id="$BLOCKER_ID"
```

Every arrow in the DAG becomes a blocked-by link. State it in the body too.

Apply labels with `gh issue edit <n> -R "$R" --add-label '<label>'`. A needed
label that does not exist is a question for the user. Never create a label
yourself.

## 7. Check

Read every posted body back:

```sh
gh issue view <n> -R "$R" --json body --jq .body
```

Check that code spans, diagrams, and `#N` references survived. Repost from the
file if not.

## 8. Report

One message: the epics with numbers and priorities, the DAG with real
numbers, the labels applied, and the counts of created and retitled issues.
