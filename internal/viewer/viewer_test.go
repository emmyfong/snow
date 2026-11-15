package viewer

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestViewerLargeFileCapped: a file past the line or byte bound shows its
// start and says what it left out.
func TestViewerLargeFileCapped(t *testing.T) {
	var b strings.Builder
	for i := range MaxLines + 500 {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	d := Parse([]byte(b.String()), int64(b.Len()))
	if !d.Truncated || len(d.Lines) != MaxLines {
		t.Fatalf("truncated %v, %d lines; want true and %d", d.Truncated, len(d.Lines), MaxLines)
	}
	if !strings.Contains(d.Limit, "first") || !strings.Contains(d.Limit, "lines") {
		t.Fatalf("limit %q, want the line limit", d.Limit)
	}

	d = Parse(bytes.Repeat([]byte("a"), MaxBytes+1), int64(MaxBytes)*3)
	if !d.Truncated || !strings.Contains(d.Limit, "MiB") {
		t.Fatalf("big file: truncated %v, limit %q; want a MiB limit", d.Truncated, d.Limit)
	}
}

// TestViewerBinary: bytes with a NUL are a binary file and are not shown.
func TestViewerBinary(t *testing.T) {
	d := Parse([]byte("ELF\x00\x01\x02"), 6)
	if !d.Binary || len(d.Lines) != 0 || d.Limit == "" {
		t.Fatalf("binary %v, %d lines, limit %q; want a notice", d.Binary, len(d.Lines), d.Limit)
	}
}

// TestViewerChangeEvidence: a recorded edit that still matches marks its
// added and removed lines and names its turn.
func TestViewerChangeEvidence(t *testing.T) {
	lines := []string{"package main", "", "func main() {", `	println("new")`, "}"}
	out, attribution, stale := Apply(lines, []Evidence{{
		Turn: 3, Tool: "edit",
		Old: "\tprintln(\"old\")",
		New: "\tprintln(\"new\")",
	}})
	if stale || attribution != "edit, turn 3" {
		t.Fatalf("stale %v, attribution %q", stale, attribution)
	}
	var plus, minus int
	for _, l := range out {
		switch l.Mark {
		case "+":
			plus++
			if l.Number != 4 || l.Text != `	println("new")` {
				t.Fatalf("added line %+v, want line 4", l)
			}
		case "-":
			minus++
			if l.Number != 0 || l.Text != `	println("old")` {
				t.Fatalf("removed line %+v", l)
			}
		}
	}
	if plus != 1 || minus != 1 {
		t.Fatalf("%d added, %d removed; want one each", plus, minus)
	}
}

// TestViewerWriteEvidence: a recorded write marks the content it wrote when
// the file still has it.
func TestViewerWriteEvidence(t *testing.T) {
	out, attribution, stale := Apply([]string{"alpha", "beta"}, []Evidence{
		{Turn: 1, Tool: "write", New: "alpha\nbeta\n"},
	})
	if stale || attribution != "write, turn 1" {
		t.Fatalf("stale %v, attribution %q", stale, attribution)
	}
	for _, l := range out {
		if l.Mark != "+" {
			t.Fatalf("line %+v, want an added mark", l)
		}
	}
}

// TestViewerStaleMarks: evidence whose text no longer matches, or matches
// more than once, leaves no marks and no attribution.
func TestViewerStaleMarks(t *testing.T) {
	out, attribution, stale := Apply([]string{"changed", "text"}, []Evidence{
		{Turn: 2, Tool: "edit", Old: "old", New: "new"},
	})
	if !stale || attribution != "" {
		t.Fatalf("stale %v, attribution %q; want stale and no attribution", stale, attribution)
	}
	for _, l := range out {
		if l.Mark != "" {
			t.Fatalf("line %+v has a mark on stale evidence", l)
		}
	}

	_, attribution, stale = Apply([]string{"same", "same"}, []Evidence{
		{Turn: 2, Tool: "edit", Old: "old", New: "same"},
	})
	if !stale || attribution != "" {
		t.Fatalf("ambiguous match: stale %v, attribution %q", stale, attribution)
	}
}
