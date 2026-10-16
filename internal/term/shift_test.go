package term

import (
	"fmt"
	"strings"
	"testing"
)

// TestHistoryCountsDroppedLines: once the scrollback is full, each new line
// drops the oldest. Dropped counts them, so history line 0 is always line
// number Dropped of everything the program printed.
func TestHistoryCountsDroppedLines(t *testing.T) {
	f := newFakePTY()
	p := newPane(f, 80, 24, Options{Scrollback: 20})
	t.Cleanup(func() { _ = p.Close() })
	next := 0
	write := func(n int) {
		var b strings.Builder
		for range n {
			fmt.Fprintf(&b, "line %d\r\n", next)
			next++
		}
		f.shellWrites(b.String())
		last := fmt.Sprintf("line %d", next-1)
		eventually(t, last, func() bool { return strings.Contains(p.Text(), last) })
	}
	write(30) // 7 lines scroll off; the scrollback is not full yet
	for range 8 {
		write(5)
		lines, _, dropped := p.History(0, 1)
		if got := lineNumber(t, ansiStripped(lines[0])); got != dropped {
			t.Fatalf("history starts at line %d, but Dropped is %d", got, dropped)
		}
	}
	if _, _, dropped := p.History(0, 1); dropped == 0 {
		t.Fatal("Dropped stayed 0 past the scrollback limit")
	}
}

// TestDroppedOvercountsWhenTheMarkIsLost: a burst longer than the whole
// scrollback drops every line copy mode knew; Dropped must then grow by at
// least that many, so a client throws its cache away.
func TestDroppedOvercountsWhenTheMarkIsLost(t *testing.T) {
	f := newFakePTY()
	p := newPane(f, 80, 24, Options{Scrollback: 20})
	t.Cleanup(func() { _ = p.Close() })
	writeLines(t, p, f, 30)
	lines, _, before := p.History(0, 1)
	first := lineNumber(t, ansiStripped(lines[0]))
	var b strings.Builder
	for i := range 200 {
		fmt.Fprintf(&b, "burst %d\r\n", i)
	}
	f.shellWrites(b.String())
	eventually(t, "burst 199", func() bool { return strings.Contains(p.Text(), "burst 199") })
	_, _, after := p.History(0, 1)
	if after-before < 20 {
		t.Fatalf("Dropped grew by %d after a burst that dropped all %d lines from line %d", after-before, 20, first)
	}
}
