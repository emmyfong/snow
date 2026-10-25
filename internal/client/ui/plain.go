package ui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// PlainText drops escape sequences and control characters other than
// line breaks from s, and turns tabs into spaces. Use it on any text from
// a model, a tool, or a vendor before styling it, so that text cannot
// restyle, move, or clear the screen, or reach the clipboard.
func PlainText(s string) string {
	s = ansi.Strip(strings.ReplaceAll(s, "\r\n", "\n"))
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// PlainLine is PlainText for one row: line breaks become spaces too.
func PlainLine(s string) string {
	return strings.ReplaceAll(PlainText(s), "\n", " ")
}
