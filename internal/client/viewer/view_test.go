package viewer

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// key is a key with text, such as "g".
func key(text string) tea.Key { return tea.Key{Text: text} }

// states are the viewer screens the goldens pin: an unchanged file, one
// whose recorded edit still matches, one with no evidence to attribute, and
// a binary file.
func states() map[string]api.ViewerState {
	return map[string]api.ViewerState{
		"Ordinary": {
			Folder: "/home/u/snow",
			Path:   "internal/viewer/read.go",
			Lines: []api.ViewerLine{
				{Number: 1, Text: "package viewer"},
				{Number: 2, Text: ""},
				{Number: 3, Text: "import ("},
				{Number: 4, Text: `	"fmt"`},
				{Number: 5, Text: ")"},
			},
		},
		"Changed": {
			Folder:      "/home/u/snow",
			Path:        "internal/viewer/read.go",
			Attribution: "edit, turn 3",
			Lines: []api.ViewerLine{
				{Number: 1, Text: "package viewer"},
				{Number: 2, Text: ""},
				{Number: 3, Text: `	"fmt"`, Mark: "+"},
				{Text: `	"os"`, Mark: "-"},
				{Number: 4, Text: ")"},
			},
		},
		"UnknownAttribution": {
			Folder: "/home/u/snow",
			Path:   "internal/viewer/marks.go",
			Lines: []api.ViewerLine{
				{Number: 1, Text: "package viewer"},
				{Number: 2, Text: ""},
				{Number: 3, Text: "// Line is one row."},
			},
		},
		"Binary": {
			Folder: "/home/u/snow",
			Path:   "testdata/logo.bin",
			Binary: true,
			Limit:  "Binary file: 6 bytes.",
		},
	}
}

// TestViewerGolden draws each viewer state at both sizes.
func TestViewerGolden(t *testing.T) {
	for name, st := range states() {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m := New(7, theme.Default())
				m.SetSize(size[0], size[1])
				m.SetState(st)
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}

// TestViewerRevealScrolls: a reveal shows the line near the top, and the
// keys move the text inside the file.
func TestViewerRevealScrolls(t *testing.T) {
	m := New(7, theme.Default())
	m.SetSize(80, 10)
	lines := make([]api.ViewerLine, 100)
	for i := range lines {
		lines[i] = api.ViewerLine{Number: i + 1, Text: fmt.Sprintf("line %d", i+1)}
	}
	m.SetState(api.ViewerState{Path: "big.go", Lines: lines, Reveal: 60})
	if m.top != 57 {
		t.Fatalf("top %d after reveal, want 57", m.top)
	}
	m.Key(key("g"))
	if m.top != 0 {
		t.Fatalf("top %d after g, want 0", m.top)
	}
}

// TestViewerBinaryTakesNoHighlight: a binary state draws its notice and no
// file lines.
func TestViewerBinaryTakesNoHighlight(t *testing.T) {
	m := New(7, theme.Default())
	m.SetSize(80, 24)
	m.SetState(api.ViewerState{Path: "logo.bin", Binary: true, Limit: "Binary file: 6 bytes."})
	if len(m.styled) != 0 || len(m.bodyRows()) != 0 {
		t.Fatalf("%d styled rows, %d body rows; want none", len(m.styled), len(m.bodyRows()))
	}
}
