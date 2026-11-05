package git

import (
	"context"
	"strings"
	"unicode/utf8"
)

// Diff bounds: a pane shows a diff, it does not page through a huge one.
const (
	MaxDiffLines = 2000
	MaxLineBytes = 1000
)

// LineKind is what one row of a diff is.
type LineKind int

// The kinds of diff row.
const (
	LineKeep   LineKind = iota // context, the same on both sides
	LineAdd                    // only in the new file
	LineRemove                 // only in the old file
	LineHunk                   // a hunk header: "@@ -1,3 +1,4 @@ func f()"
)

// Line is one row of a diff, without its leading "+", "-", or space.
type Line struct {
	Kind LineKind
	Text string
}

// FileDiff is the diff of one file.
type FileDiff struct {
	Lines  []Line
	Binary bool // git does not show a binary file's change
	Cut    bool // past MaxDiffLines; the rest is left out
}

// Diff returns c's diff: the staged change when staged is set, else the
// unstaged one. An untracked file shows as added whole.
func (r Repo) Diff(ctx context.Context, c Change, staged bool) (FileDiff, error) {
	args := []string{"diff", "--no-color", "--no-ext-diff", "--no-textconv", "--unified=3"}
	dir := r.Path
	switch {
	case c.Untracked() && !staged:
		// --no-index reads paths from the folder git runs in, not the top.
		top, err := r.Run(ctx, r.Path, "rev-parse", "--show-toplevel")
		if err != nil {
			return FileDiff{}, r.Classify(err)
		}
		dir = strings.TrimSpace(top)
		args = append(args, "--no-index", "--", "/dev/null", c.Path)
	case staged:
		args = append(args, "--cached", "--")
		if c.Orig != "" {
			args = append(args, pathspec(c.Orig))
		}
		args = append(args, pathspec(c.Path))
	default:
		args = append(args, "--", pathspec(c.Path))
	}
	out, err := r.Run(ctx, dir, args...)
	// --no-index exits 1 when the files differ, which they always do here.
	if err != nil && (!c.Untracked() || !Declined(err)) {
		return FileDiff{}, r.Classify(err)
	}
	return parseDiff(out), nil
}

// parseDiff reads unified diff output. Headers before the first hunk, such
// as "diff --git" and "+++ b/x", are not rows.
func parseDiff(out string) FileDiff {
	var d FileDiff
	inHunk := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSuffix(line, "\r")
		var l Line
		switch {
		case strings.HasPrefix(line, "diff --git "), strings.HasPrefix(line, "diff --cc "):
			inHunk = false
			continue
		case strings.HasPrefix(line, "Binary files ") && !inHunk:
			d.Binary = true
			continue
		case strings.HasPrefix(line, "@@"):
			inHunk = true
			l = Line{Kind: LineHunk, Text: line}
		case !inHunk || line == "":
			continue
		case line[0] == '+':
			l = Line{Kind: LineAdd, Text: line[1:]}
		case line[0] == '-':
			l = Line{Kind: LineRemove, Text: line[1:]}
		case line[0] == ' ':
			l = Line{Kind: LineKeep, Text: line[1:]}
		default:
			continue // "\ No newline at end of file"
		}
		if len(d.Lines) == MaxDiffLines {
			d.Cut = true
			break
		}
		l.Text = cut(l.Text, MaxLineBytes)
		d.Lines = append(d.Lines, l)
	}
	return d
}

// cut returns the first n bytes of s, ending on a whole rune, as valid
// UTF-8.
func cut(s string, n int) string {
	if len(s) > n {
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		s = s[:n]
	}
	return strings.ToValidUTF8(s, "�")
}
