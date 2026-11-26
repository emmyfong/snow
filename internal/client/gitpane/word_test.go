package gitpane

import (
	"image/color"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestChangeWords(t *testing.T) {
	th := testTheme()
	for _, tt := range []struct {
		code, word string
		fg         color.Color
	}{
		{"A", "added", th.Added}, {"M", "modified", th.Modified},
		{"T", "modified", th.Modified}, {"D", "deleted", th.Removed},
		{"R", "renamed", th.Accent}, {"C", "copied", th.Accent},
		{"U", "conflict", th.ErrorFG}, {"?", "untracked", th.Muted},
		{"X", "changed", th.Muted}, {"", "changed", th.Muted},
	} {
		for _, staged := range []bool{false, true} {
			for _, conflict := range []bool{false, true} {
				e := entry{change: api.GitChange{Index: tt.code, Work: tt.code, Conflict: conflict}, staged: staged}
				if staged {
					e.change.Work = "D"
				} else {
					e.change.Index = "A"
				}
				word, fg, bg := changeWord(th, e)
				want, wantFG := tt.word, tt.fg
				var wantBG color.Color
				if conflict || tt.code == "U" {
					want, wantFG, wantBG = "conflict", th.ErrorFG, th.Error
				}
				if word != want || fg != wantFG || bg != wantBG {
					t.Errorf("code %q staged %v conflict %v: (%q, %v, %v), want (%q, %v, %v)", tt.code, staged, conflict, word, fg, bg, want, wantFG, wantBG)
				}
			}
		}
	}
}
