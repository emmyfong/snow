package viewer

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/emmyfong/snow/internal/client/theme"
)

// tokenLines highlights text and returns one styled string per line. A
// text without a known lexer comes back as its lines, unstyled.
func tokenLines(name, text string, th theme.Theme) []string {
	lexer := lexers.Match(name)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return strings.Split(text, "\n")
	}
	out := make([]string, 0, strings.Count(text, "\n")+1)
	var line strings.Builder
	tok := it()
	for tok != chroma.EOF {
		style := tokenStyle(tok.Type, th)
		parts := strings.Split(tok.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				out = append(out, line.String())
				line.Reset()
			}
			if part != "" {
				line.WriteString(style.Render(part))
			}
		}
		tok = it()
	}
	out = append(out, line.String())
	return out
}

// tokenStyle maps a chroma token to one of Snow's theme colors. It never
// sets a background, so the pane keeps the terminal's own.
func tokenStyle(t chroma.TokenType, th theme.Theme) lipgloss.Style {
	switch {
	case t.InCategory(chroma.Comment):
		return lipgloss.NewStyle().Foreground(th.Muted)
	case t.InCategory(chroma.Keyword):
		return lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	case t.InSubCategory(chroma.LiteralString):
		return lipgloss.NewStyle().Foreground(th.Added)
	case t.InSubCategory(chroma.LiteralNumber):
		return lipgloss.NewStyle().Foreground(th.Accent)
	case t.InSubCategory(chroma.NameFunction), t.InSubCategory(chroma.NameClass), t.InSubCategory(chroma.NameBuiltin):
		return lipgloss.NewStyle().Foreground(th.Accent)
	case t.InSubCategory(chroma.GenericHeading), t.InSubCategory(chroma.GenericSubheading):
		return lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	case t == chroma.Error:
		return lipgloss.NewStyle().Foreground(th.Removed)
	}
	return lipgloss.NewStyle()
}
