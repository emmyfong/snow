package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/theme"
)

// TextSpan is a run of text to draw as a link: Start runes in, Len long.
type TextSpan struct {
	Start, Len int
}

// LinkText draws text with spans in the theme's link colors. Spans that
// run past the end of text, overlap, or are empty are ignored. The text
// must already be plain: LinkText adds escape sequences, so it runs last.
func LinkText(th theme.Theme, text string, spans []TextSpan) string {
	if text == "" || len(spans) == 0 {
		return text
	}
	style := lipgloss.NewStyle().Foreground(th.Accent).Underline(true)
	runes := []rune(text)
	var b strings.Builder
	end := 0
	for _, s := range spans {
		start := max(s.Start, 0)
		stop := min(start+s.Len, len(runes))
		if start < end || start >= stop {
			continue
		}
		b.WriteString(string(runes[end:start]))
		b.WriteString(style.Render(string(runes[start:stop])))
		end = stop
	}
	b.WriteString(string(runes[end:]))
	return b.String()
}
