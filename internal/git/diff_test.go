package git

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestParseDiff: headers are dropped; hunks, context, adds, and removes
// stay, without their first character.
func TestParseDiff(t *testing.T) {
	out := strings.Join([]string{
		"diff --git a/x.go b/x.go",
		"index 111..222 100644",
		"--- a/x.go",
		"+++ b/x.go",
		"@@ -1,3 +1,3 @@ func f()",
		" keep",
		"-old",
		"+new",
		"\\ No newline at end of file",
		"",
	}, "\n")
	want := FileDiff{Lines: []Line{
		{LineHunk, "@@ -1,3 +1,3 @@ func f()"},
		{LineKeep, "keep"},
		{LineRemove, "old"},
		{LineAdd, "new"},
	}}
	if got := parseDiff(out); !reflect.DeepEqual(got, want) {
		t.Errorf("parseDiff:\n got %+v\nwant %+v", got, want)
	}
}

// TestParseDiffBinary: a binary file says so and has no rows.
func TestParseDiffBinary(t *testing.T) {
	d := parseDiff("diff --git a/i.png b/i.png\nindex 1..2 100644\nBinary files a/i.png and b/i.png differ\n")
	if !d.Binary || len(d.Lines) != 0 {
		t.Errorf("%+v", d)
	}
}

// TestParseDiffBounds: past MaxDiffLines the diff is cut, and a long line
// is cut on a whole rune.
func TestParseDiffBounds(t *testing.T) {
	var b strings.Builder
	b.WriteString("@@ -0,0 +1 @@\n")
	for i := 0; i < MaxDiffLines+5; i++ {
		b.WriteString("+x\n")
	}
	if d := parseDiff(b.String()); !d.Cut || len(d.Lines) != MaxDiffLines {
		t.Errorf("%d lines, cut %v", len(d.Lines), d.Cut)
	}
	long := "+" + strings.Repeat("é", MaxLineBytes)
	d := parseDiff("@@ -0,0 +1 @@\n" + long + "\n")
	if got := d.Lines[1].Text; len(got) > MaxLineBytes || !utf8.ValidString(got) || got == "" {
		t.Errorf("long line cut to %d bytes, valid %v", len(got), utf8.ValidString(got))
	}
}

// TestCutInvalid: invalid UTF-8 inside a line does not empty it.
func TestCutInvalid(t *testing.T) {
	if got := cut("a\xffb", 10); got != "a�b" {
		t.Errorf("cut = %q", got)
	}
}

// TestGitDiffReal: unstaged, staged, and untracked diffs from real git.
func TestGitDiffReal(t *testing.T) {
	ctx := context.Background()
	dir := repo(t)
	r := Repo{Path: dir}
	write(t, dir, "a.txt", "b\n")
	d, err := r.Diff(ctx, Change{Path: "a.txt", Work: "M"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !hasLine(d, LineRemove, "a") || !hasLine(d, LineAdd, "b") {
		t.Errorf("unstaged diff %+v", d)
	}
	if err := r.Stage(ctx, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if d, err = r.Diff(ctx, Change{Path: "a.txt", Index: "M"}, true); err != nil || !hasLine(d, LineAdd, "b") {
		t.Errorf("staged diff %+v, %v", d, err)
	}
	write(t, dir, "sub/new.txt", "fresh\n")
	sub := Repo{Path: dir + "/sub"}
	if d, err = sub.Diff(ctx, Change{Path: "sub/new.txt", Work: "?"}, false); err != nil || !hasLine(d, LineAdd, "fresh") {
		t.Errorf("untracked diff from a subfolder %+v, %v", d, err)
	}
}

func hasLine(d FileDiff, kind LineKind, text string) bool {
	for _, l := range d.Lines {
		if l.Kind == kind && l.Text == text {
			return true
		}
	}
	return false
}
