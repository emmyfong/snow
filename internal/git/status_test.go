package git

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/gittest"
)

// TestGitStatusParse: porcelain v2 records for modified, added, deleted,
// renamed, untracked, and conflicted files, a path with spaces, and the
// branch headers.
func TestGitStatusParse(t *testing.T) {
	out := strings.Join([]string{
		"# branch.oid 1234567890abcdef",
		"# branch.head main",
		"# branch.upstream origin/main",
		"# branch.ab +2 -1",
		"1 .M N... 100644 100644 100644 aaa aaa src/main.go",
		"1 A. N... 000000 100644 100644 000 bbb new file.txt",
		"1 D. N... 100644 000000 000000 ccc 000 gone.txt",
		"2 R. N... 100644 100644 100644 ddd ddd R100 docs/new.md", "docs/old.md",
		"u UU N... 100644 100644 100644 100644 e1 e2 e3 both.txt",
		"? notes/todo.txt",
		"",
	}, "\x00")
	got := parseStatus(out)
	want := Status{
		Branch: "main", Upstream: "origin/main", Ahead: 2, Behind: 1,
		Changes: []Change{
			{Path: "src/main.go", Work: "M"},
			{Path: "new file.txt", Index: "A"},
			{Path: "gone.txt", Index: "D"},
			{Path: "docs/new.md", Orig: "docs/old.md", Index: "R"},
			{Path: "both.txt", Index: "U", Work: "U", Conflict: true},
			{Path: "notes/todo.txt", Work: "?"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseStatus:\n got %+v\nwant %+v", got, want)
	}
}

// TestGitStatusHeaders: a detached HEAD and a branch with no commit.
func TestGitStatusHeaders(t *testing.T) {
	if st := parseStatus("# branch.oid abc\x00# branch.head (detached)\x00"); !st.Detached || st.Branch != "" {
		t.Errorf("detached: %+v", st)
	}
	if st := parseStatus("# branch.oid (initial)\x00# branch.head main\x00"); !st.Initial || st.Branch != "main" {
		t.Errorf("initial: %+v", st)
	}
}

// TestGitStatusCap: past MaxChanges the rest are counted, not listed.
func TestGitStatusCap(t *testing.T) {
	var b strings.Builder
	for i := 0; i < MaxChanges+3; i++ {
		b.WriteString("? f\x00")
	}
	st := parseStatus(b.String())
	if len(st.Changes) != MaxChanges || st.Omitted != 3 {
		t.Errorf("%d changes, %d omitted; want %d and 3", len(st.Changes), st.Omitted, MaxChanges)
	}
}

// TestGitStatusReal: real git, from a subfolder: paths are from the top.
func TestGitStatusReal(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "changed\n")
	write(t, dir, "sub/b.txt", "b\n")
	st, err := Repo{Path: dir + "/sub"}.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{{Path: "a.txt", Work: "M"}, {Path: "sub/b.txt", Work: "?"}}
	if st.Branch != "main" || !reflect.DeepEqual(st.Changes, want) {
		t.Errorf("status %+v, want main with %+v", st, want)
	}
}

// TestGitStatusEmptyRepo: a repository with no commit has a branch and no
// changes.
func TestGitStatusEmptyRepo(t *testing.T) {
	gittest.Isolate(t)
	dir := t.TempDir()
	gittest.Run(t, dir, "init", "-q", "-b", "main")
	st, err := Repo{Path: dir}.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Initial || st.Branch != "main" || len(st.Changes) != 0 {
		t.Errorf("status %+v", st)
	}
}
