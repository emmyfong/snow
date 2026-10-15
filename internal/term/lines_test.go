package term

import (
	"strings"
	"testing"
)

func TestPaneVersionChangesWithOutput(t *testing.T) {
	p, f := newTestPane(t)
	before := p.Version()
	f.shellWrites("x")
	eventually(t, "version to change", func() bool { return p.Version() != before })
	settled := p.Version()
	if p.Version() != settled {
		t.Fatal("version changed without output")
	}
}

func TestPaneLines(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("first\r\n\x1b[31msecond\x1b[0m")
	eventually(t, "output", func() bool { return strings.Contains(p.Text(), "second") })
	lines := p.Lines()
	if len(lines) != 24 {
		t.Fatalf("Lines() has %d rows, want 24", len(lines))
	}
	if !strings.Contains(lines[0], "first") || !strings.Contains(lines[1], "second") || !strings.Contains(lines[1], "\x1b[") {
		t.Fatalf("rows 0 and 1 = %q, %q; want plain and styled text", lines[0], lines[1])
	}
}
