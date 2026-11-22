# Sweep merged worktrees

A worktree and its local branch leave once the PR for that branch merges. The
agent that opened the PR stops before the merge and does not come back, so the
top-level agent runs this sweep before it starts new work. `issue-handler`,
`next-issue`, and `manage-subagents` call it. A worker that a manager launched
skips it; the manager has already swept.

## Rules

- Run one command at a time. Do not run a recursive search over `.worktrees/`.
- Never use `--force`. Never remove the main checkout.
- Keep a worktree when any check fails or any command exits with an error.
  Report it with the reason or the error message.
- If a hook blocks a command, stop and tell the user.

## 1. List the worktrees

`<root>` is the main checkout: the first path that `git worktree list` prints.
`<owner/repo>` is the value that
`gh repo view --json nameWithOwner --jq .nameWithOwner` prints. Shell variables
do not survive between calls, so write both values literally.

```sh
git -C <root> worktree list --porcelain
```

Check only worktrees whose path is under `<root>/.worktrees/`. Leave every other
path alone. Skip the worktree that holds your current directory, and report it.
Keep a worktree in detached HEAD. A detached worktree has no branch to match a PR.

## 2. Check each worktree

For each `<wt>` with branch `<branch>`:

```sh
gh pr list -R <owner/repo> --head <branch> --state all --json number,state,headRefOid
git -C <wt> status --porcelain
git -C <wt> rev-parse HEAD
```

Remove it only when all of these are true:

1. At least one PR for `<branch>` has state `MERGED`, and none has state `OPEN`.
2. `git status --porcelain` exits with status 0 and prints nothing.
3. `rev-parse HEAD` exits with status 0. HEAD equals the `headRefOid` of a
   merged PR, or is an ancestor of it. For each merged `headRefOid`:
   - `git -C <root> cat-file -e <headRefOid>^{commit}` tells whether the commit
     is in the local repository.
   - If it is, run `git -C <root> merge-base --is-ancestor <HEAD> <headRefOid>`.
     Exit status 0 passes. Any other status fails.
   - If it is not, ask GitHub:
     `gh api repos/<owner/repo>/compare/<HEAD>...<headRefOid> --jq .status`.
     The check passes on `identical` or `ahead`. An error means GitHub does
     not have HEAD, so the worktree holds unpushed commits. Keep it.

A worktree with local commits after its PR may hold follow-up work. Keep it and
report it. A worktree whose PRs all closed without a merge stays too. Report it,
so the user can decide.

The ignored `<wt>/.context/` folder leaves with the worktree. Before removal,
read the Decisions and Gotchas in `<wt>/.context/CONTEXT.md` if it exists. Report
any rule there that is not yet in `AGENTS.md` or an ADR, so the user can
graduate it with the `context` skill.

## 3. Remove

```sh
git -C <root> worktree remove <wt>
git -C <root> branch -D <branch>
```

A squash merge leaves the branch unmerged in git's view, so `-d` refuses it.
`-D` is safe only after check 1. If `worktree remove` refuses, keep the
worktree and report the message.

Do not run `git worktree prune`. It also drops the record of a worktree whose
folder cannot be read for a moment, such as on a failing `/mnt/c` drive.
`worktree remove` already clears the record of each worktree it removes.

Report each removed worktree with its PR number, and each kept worktree with its
reason.
