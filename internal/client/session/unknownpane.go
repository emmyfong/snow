package session

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// unknownPane stands in for a pane of a kind this client cannot draw, such
// as one a newer server added. It says so and takes no input; the pane
// can still be focused, moved, and closed.
type unknownPane struct {
	kind          string
	th            theme.Theme
	width, height int
}

func newUnknownPane(kind string, th theme.Theme) *unknownPane {
	return &unknownPane{kind: kind, th: th}
}

func (u *unknownPane) SetSize(width, height int) { u.width, u.height = width, height }
func (u *unknownPane) SetTheme(th theme.Theme)   { u.th = th }

// View centers two muted lines in the pane, cut to its width.
func (u *unknownPane) View() []string {
	if u.width <= 0 || u.height <= 0 {
		return nil
	}
	muted := lipgloss.NewStyle().Foreground(u.th.Resolve().Muted)
	text := []string{
		fmt.Sprintf("This Snow cannot show a %q pane.", u.kind),
		"Update Snow, or close the pane.",
	}
	rows := make([]string, u.height)
	top := max((u.height-len(text))/2, 0)
	for i, t := range text {
		if top+i >= u.height {
			break
		}
		t = ansi.Truncate(t, u.width, "…")
		pad := max((u.width-ansi.StringWidth(t))/2, 0)
		rows[top+i] = fmt.Sprintf("%*s%s", pad, "", muted.Render(t))
	}
	return rows
}

func (u *unknownPane) Cursor() (x, y int, ok bool)          { return 0, 0, false }
func (u *unknownPane) Feed(api.Message)                     {}
func (u *unknownPane) Key(tea.Key) (api.Message, string)    { return nil, "" }
func (u *unknownPane) Click(int, int) (api.Message, string) { return nil, "" }
func (u *unknownPane) Paste(string)                         {}
func (u *unknownPane) Scroll(int)                           {}
func (u *unknownPane) Animating() bool                      { return false }
func (u *unknownPane) Tick()                                {}
