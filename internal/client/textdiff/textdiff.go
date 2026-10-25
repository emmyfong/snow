// Package textdiff compares two texts line by line, for showing a change
// to a file.
package textdiff

import "strings"

// Op is what a line of a diff does.
type Op int

// Kinds of diff line.
const (
	Keep   Op = iota // in both texts
	Add              // only in the new text
	Remove           // only in the old text
)

// Line is one line of a diff, without its line break.
type Line struct {
	Op   Op
	Text string
}

// maxCells bounds the table Lines builds for the lines that differ. A
// larger change shows as all old lines removed and all new lines added:
// right, though not the shortest diff, and it never stalls the screen.
const maxCells = 1 << 20

// Lines returns the lines of before and after in order, each marked kept,
// added, or removed. The lines both texts start and end with are matched
// first, so a small edit in a large file stays cheap.
func Lines(before, after string) []Line {
	a, b := split(before), split(after)
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	out := make([]Line, 0, len(a)+len(b)-pre-suf)
	for _, l := range a[:pre] {
		out = append(out, Line{Keep, l})
	}
	out = append(out, middle(a[pre:len(a)-suf], b[pre:len(b)-suf])...)
	for _, l := range a[len(a)-suf:] {
		out = append(out, Line{Keep, l})
	}
	return out
}

// Counts returns how many lines lines adds and removes.
func Counts(lines []Line) (added, removed int) {
	for _, l := range lines {
		switch l.Op {
		case Add:
			added++
		case Remove:
			removed++
		}
	}
	return added, removed
}

// split cuts s into lines. A final line break ends the last line; it does
// not start an empty one.
func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
}

// middle diffs a and b by their longest common subsequence.
func middle(a, b []string) []Line {
	var out []Line
	if len(a) == 0 || len(b) == 0 || (len(a)+1)*(len(b)+1) > maxCells {
		for _, l := range a {
			out = append(out, Line{Remove, l})
		}
		for _, l := range b {
			out = append(out, Line{Add, l})
		}
		return out
	}
	// lcs[i][j] is the length of the longest common subsequence of a[i:]
	// and b[j:].
	w := len(b) + 1
	lcs := make([]int32, (len(a)+1)*w)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i*w+j] = lcs[(i+1)*w+j+1] + 1
			} else {
				lcs[i*w+j] = max(lcs[(i+1)*w+j], lcs[i*w+j+1])
			}
		}
	}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, Line{Keep, a[i]})
			i, j = i+1, j+1
		case lcs[(i+1)*w+j] >= lcs[i*w+j+1]:
			out = append(out, Line{Remove, a[i]})
			i++
		default:
			out = append(out, Line{Add, b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		out = append(out, Line{Remove, a[i]})
	}
	for ; j < len(b); j++ {
		out = append(out, Line{Add, b[j]})
	}
	return out
}
