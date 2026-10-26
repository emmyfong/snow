package convstore

import (
	"strings"
	"unicode"
)

// OneLine makes text a single line: control characters, tabs, and line
// breaks become spaces, and runs of whitespace become one space. A title
// shown on a screen goes through it, so a conversation file cannot carry
// an escape sequence to the terminal.
func OneLine(text string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)), " ")
}

// Title returns the conversation's title as the history list shows it: the
// last rename, else the title in the meta record, else the start of the
// first message. A conversation with none has an empty title.
func (c *Conversation) Title() string {
	title, renamed, first := "", false, ""
	for _, r := range c.Records {
		switch r := r.(type) {
		case *User:
			if first == "" {
				first = titleFrom(r.Text)
			}
		case *Rename:
			title, renamed = r.Title, true
		}
	}
	switch {
	case renamed:
		return title
	case c.Meta.Title != "":
		return c.Meta.Title
	}
	return first
}
