package worktree

import (
	"reflect"
	"testing"
)

// TestParseList: the porcelain output of git worktree list, with a branch,
// a detached HEAD, a locked worktree, a bare repository, and Windows line
// endings.
func TestParseList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Worktree
	}{
		{"main and a branch", "worktree /r\nHEAD aaa\nbranch refs/heads/main\n\nworktree /r/.worktrees/x\nHEAD bbb\nbranch refs/heads/feat/x\n\n", []Worktree{
			{Path: "/r", Head: "aaa", Branch: "main", Main: true},
			{Path: "/r/.worktrees/x", Head: "bbb", Branch: "feat/x"},
		}},
		{"detached and locked", "worktree /r\nHEAD aaa\nbranch refs/heads/main\n\nworktree /w\nHEAD bbb\ndetached\nlocked in use\n\n", []Worktree{
			{Path: "/r", Head: "aaa", Branch: "main", Main: true},
			{Path: "/w", Head: "bbb", Detached: true, Locked: true},
		}},
		{"bare", "worktree /r.git\nbare\n\n", []Worktree{{Path: "/r.git", Main: true, Bare: true}}},
		{"windows line endings", "worktree C:/r\r\nHEAD aaa\r\nbranch refs/heads/main\r\n\r\n", []Worktree{{Path: "C:/r", Head: "aaa", Branch: "main", Main: true}}},
		{"path with spaces", "worktree /my repo\nHEAD aaa\nbranch refs/heads/main\n\n", []Worktree{{Path: "/my repo", Head: "aaa", Branch: "main", Main: true}}},
		{"nothing", "", nil},
		{"a line before any worktree", "HEAD aaa\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseList(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseList = %+v, want %+v", got, tt.want)
			}
		})
	}
}
