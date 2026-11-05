package api

import (
	"errors"
	"testing"
)

func TestGitStateValidate(t *testing.T) {
	tests := []struct {
		name string
		s    GitState
		ok   bool
	}{
		{"empty", GitState{}, true},
		{"whole", GitState{
			Changes:   []GitChange{{Path: "a"}},
			Diff:      &GitDiff{Path: "a"},
			Worktrees: []GitWorktree{{Path: "/r"}},
			History:   []GitCommit{{Hash: "h"}},
		}, true},
		{"negative ahead", GitState{Ahead: -1}, false},
		{"negative omitted", GitState{Omitted: -1}, false},
		{"change with no path", GitState{Changes: []GitChange{{}}}, false},
		{"diff with no path", GitState{Diff: &GitDiff{}}, false},
		{"worktree with no path", GitState{Worktrees: []GitWorktree{{}}}, false},
		{"commit with no hash", GitState{History: []GitCommit{{Subject: "x"}}}, false},
	}
	for _, tt := range tests {
		err := tt.s.Validate()
		if (err == nil) != tt.ok {
			t.Errorf("%s: Validate() = %v, want ok %v", tt.name, err, tt.ok)
		}
		if err != nil && !errors.Is(err, ErrInvalidSidePane) {
			t.Errorf("%s: %v is not ErrInvalidSidePane", tt.name, err)
		}
	}
}
