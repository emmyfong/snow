package claudecode

import (
	"fmt"
	"strings"
)

// diffContext is how many unchanged lines a hunk shows around a change.
const diffContext = 3

// unifiedDiff returns the change from a to b as a unified diff with the
// file names nameA and nameB, or "" when they are the same. Settings files
// are small, so a plain longest-common-subsequence table is fast enough.
func unifiedDiff(a, b []byte, nameA, nameB string) string {
	x, y := diffLines(a), diffLines(b)
	ops := diffOps(x, y)
	var out strings.Builder
	for start := 0; start < len(ops); {
		// Find the next change, then the end of its hunk: changes closer
		// than twice the context share a hunk.
		first := start
		for first < len(ops) && ops[first].kind == ' ' {
			first++
		}
		if first == len(ops) {
			break
		}
		last := first
		for i := first; i < len(ops); i++ {
			if ops[i].kind != ' ' {
				last = i
			} else if i-last > 2*diffContext {
				break
			}
		}
		from, to := max(first-diffContext, 0), min(last+diffContext+1, len(ops))
		if out.Len() == 0 {
			fmt.Fprintf(&out, "--- %s\n+++ %s\n", nameA, nameB)
		}
		writeHunk(&out, ops[from:to])
		start = to
	}
	return out.String()
}

// diffLines splits data into lines, without their line endings.
func diffLines(data []byte) []string {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// op is one line of a diff: ' ' kept, '-' only in a, '+' only in b, with
// its line numbers, counted from 1.
type op struct {
	kind   byte
	text   string
	ai, bi int
}

// diffOps returns the lines of a and b in order, each kept, removed, or
// added.
func diffOps(a, b []string) []op {
	// lcs[i][j] is the longest common subsequence of a[i:] and b[j:].
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []op
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			ops = append(ops, op{' ', a[i], i + 1, j + 1})
			i, j = i+1, j+1
		// On a tie the removal comes first, as diff tools show it.
		case j < len(b) && (i == len(a) || lcs[i][j+1] > lcs[i+1][j]):
			ops = append(ops, op{'+', b[j], i, j + 1})
			j++
		default:
			ops = append(ops, op{'-', a[i], i + 1, j})
			i++
		}
	}
	return ops
}

// writeHunk writes one hunk: its header with the ranges in a and b, then
// its lines.
func writeHunk(out *strings.Builder, ops []op) {
	aStart, bStart, aLen, bLen := 0, 0, 0, 0
	for _, o := range ops {
		if o.kind != '+' {
			if aLen == 0 {
				aStart = o.ai
			}
			aLen++
		}
		if o.kind != '-' {
			if bLen == 0 {
				bStart = o.bi
			}
			bLen++
		}
	}
	if aLen == 0 {
		aStart = ops[0].ai
	}
	if bLen == 0 {
		bStart = ops[0].bi
	}
	fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n", aStart, aLen, bStart, bLen)
	for _, o := range ops {
		out.WriteByte(o.kind)
		out.WriteString(o.text)
		out.WriteByte('\n')
	}
}
