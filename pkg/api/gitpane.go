package api

import (
	"fmt"
	"strconv"
)

// PaneGit is the git pane: the changes in the folder it opened from, their
// diffs, and stage and commit actions. Its state is GitState.
const PaneGit = "git"

// The actions of a git pane, in SidePaneAction.Action.
const (
	GitActRefresh   = "refresh"    // read the folder's status again
	GitActStage     = "stage"      // stage ArgGitPath
	GitActStageAll  = "stage-all"  // stage every change
	GitActUnstage   = "unstage"    // take ArgGitPath back out of the index
	GitActCommit    = "commit"     // commit what is staged with ArgGitMessage
	GitActDiff      = "diff"       // show ArgGitPath's diff; ArgGitStaged picks the staged one
	GitActCloseDiff = "close-diff" // go back to the list of changes
)

// The arguments of a git pane's actions.
const (
	ArgGitPath    = "path"    // a changed file, from the top of the repository
	ArgGitStaged  = "staged"  // "true" for the staged change
	ArgGitMessage = "message" // a commit message
)

// MaxCommitMessage bounds a commit message the git pane sends.
const MaxCommitMessage = 16 << 10

// GitState is the state of a git pane.
type GitState struct {
	Folder  string `json:"folder"`
	Loading bool   `json:"loading,omitempty"` // git runs; the rest is the last answer
	// NotRepo means the folder is in no git repository. The rest is empty.
	NotRepo  bool        `json:"notRepo,omitempty"`
	Branch   string      `json:"branch,omitempty"` // empty on a detached HEAD
	Detached bool        `json:"detached,omitempty"`
	Initial  bool        `json:"initial,omitempty"` // the branch has no commit yet
	Upstream string      `json:"upstream,omitempty"`
	Ahead    int         `json:"ahead,omitempty"`
	Behind   int         `json:"behind,omitempty"`
	Changes  []GitChange `json:"changes,omitempty"`
	Omitted  int         `json:"omitted,omitempty"` // changes left out of Changes
	Diff     *GitDiff    `json:"diff,omitempty"`    // the open diff, or nil
	// Worktrees are the repository's checkouts, the main one first. It
	// is empty when git cannot list them.
	Worktrees []GitWorktree `json:"worktrees,omitempty"`
	// Error is the last action's failure, in git's own words. Notice is
	// the last action's outcome, such as the commit it made.
	Error  string `json:"error,omitempty"`
	Notice string `json:"notice,omitempty"`
}

// Kind implements SideState.
func (*GitState) Kind() string { return PaneGit }

// Validate implements SideState.
func (s *GitState) Validate() error {
	if s.Ahead < 0 || s.Behind < 0 || s.Omitted < 0 {
		return fmt.Errorf("%w: negative count in git state", ErrInvalidSidePane)
	}
	for _, c := range s.Changes {
		if c.Path == "" {
			return fmt.Errorf("%w: git change with no path", ErrInvalidSidePane)
		}
	}
	if s.Diff != nil && s.Diff.Path == "" {
		return fmt.Errorf("%w: git diff with no path", ErrInvalidSidePane)
	}
	for _, w := range s.Worktrees {
		if w.Path == "" {
			return fmt.Errorf("%w: git worktree with no path", ErrInvalidSidePane)
		}
	}
	return nil
}

// GitWorktree is one checkout of a repository, in the repository's own
// path form: a Linux path for a WSL repository.
type GitWorktree struct {
	Path     string `json:"path"`
	Branch   string `json:"branch,omitempty"` // empty for a detached HEAD
	Main     bool   `json:"main,omitempty"`   // the repository's own checkout
	Detached bool   `json:"detached,omitempty"`
	Locked   bool   `json:"locked,omitempty"`
}

// GitChange is one changed file. Index and Work are git's one-letter codes
// for the staged and the unstaged change (M, A, D, R, C, T, U), empty for
// none; an untracked file has Work "?".
type GitChange struct {
	Path     string `json:"path"`
	Orig     string `json:"orig,omitempty"` // a rename's old path
	Index    string `json:"index,omitempty"`
	Work     string `json:"work,omitempty"`
	Conflict bool   `json:"conflict,omitempty"`
}

// Staged reports whether c has a staged change.
func (c GitChange) Staged() bool { return c.Index != "" && !c.Conflict }

// Unstaged reports whether c has a change that is not staged, an untracked
// file included.
func (c GitChange) Unstaged() bool { return c.Work != "" }

// GitDiff is the diff of one file.
type GitDiff struct {
	Path   string        `json:"path"`
	Staged bool          `json:"staged,omitempty"`
	Lines  []GitDiffLine `json:"lines,omitempty"`
	Binary bool          `json:"binary,omitempty"`
	Cut    bool          `json:"cut,omitempty"` // rows were left out at the end
}

// The kinds of GitDiffLine.
const (
	GitLineKeep   = "keep"
	GitLineAdd    = "add"
	GitLineRemove = "remove"
	GitLineHunk   = "hunk" // "@@ -1,3 +1,4 @@ ..."
)

// GitDiffLine is one row of a diff, without its leading "+", "-", or space.
type GitDiffLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// GitStagedArg is the ArgGitStaged value for staged.
func GitStagedArg(staged bool) string { return strconv.FormatBool(staged) }
