package term

import "testing"

// TestCursorVisibility: programs such as editors and progress bars hide the
// cursor with DECTCEM; clients must not draw it then.
func TestCursorVisibility(t *testing.T) {
	p, f := newTestPane(t)
	if _, _, visible := p.Cursor(); !visible {
		t.Fatal("cursor hidden at start")
	}
	f.shellWrites("\x1b[?25lhidden")
	eventually(t, "hidden cursor", func() bool { _, _, v := p.Cursor(); return !v })
	f.shellWrites("\x1b[?25hshown")
	eventually(t, "shown cursor", func() bool { _, _, v := p.Cursor(); return v })
}
