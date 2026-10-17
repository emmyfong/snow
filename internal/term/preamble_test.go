package term

import (
	"strings"
	"testing"
)

// TestPreambleComesFirst: restored lines are on screen before the program
// prints anything, so its output follows them.
func TestPreambleComesFirst(t *testing.T) {
	f := newFakePTY()
	p := newPane(f, 80, 24, Options{Preamble: "saved line one\r\nsaved line two\r\n"})
	t.Cleanup(func() { _ = p.Close() })
	f.shellWrites("prompt$ ")
	eventually(t, "prompt", func() bool { return strings.Contains(p.Text(), "prompt$") })
	text := p.Text()
	one, two, prompt := strings.Index(text, "saved line one"), strings.Index(text, "saved line two"), strings.Index(text, "prompt$")
	if one < 0 || two < one || prompt < two {
		t.Fatalf("screen:\n%s\nwant the saved lines, then the prompt", text)
	}
}
