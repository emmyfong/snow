package term

import (
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// Match is one place a search found its query in the history: the line
// (numbered as in History), the cell column where the match starts, and its
// width in cells.
type Match struct {
	Line, Col, Len int
}

// MaxMatches bounds how many matches Search returns.
const MaxMatches = 5000

// Search finds every non-overlapping occurrence of query in the history,
// scrollback and screen, in line order and, within a line, left to right.
// The query is plain text, not a pattern. The match ignores case unless the
// query has an upper-case letter. With more than MaxMatches, it keeps the
// newest MaxMatches, nearest the screen, and sets truncated. Dropped is as in
// History, for the same lines. An empty query matches nothing.
func (p *Pane) Search(query string) (matches []Match, truncated bool, dropped int) {
	matches, _, truncated, dropped = p.search(query)
	return matches, truncated, dropped
}

// SearchLines returns the plain text of every line Search finds query in, in
// line order, one entry per match, without the blank cells that pad a screen
// row. The text is copied under the same lock as the search, so each one
// belongs to the line that matched: a line that scrolls away between two
// calls cannot report another line's text.
func (p *Pane) SearchLines(query string) []string {
	matches, lines, _, _ := p.search(query)
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = strings.TrimRight(lines[m.Line], " ")
	}
	return out
}

// search finds the matches of query and keeps the plain lines they came
// from, so a caller can read a matched line under the same lock.
func (p *Pane) search(query string) (matches []Match, lines []string, truncated bool, dropped int) {
	q := []rune(query)
	if len(q) == 0 {
		return nil, nil, false, 0
	}
	fold := true
	for _, r := range q {
		if unicode.IsUpper(r) {
			fold = false
		}
	}
	lines, dropped = p.plainHistory()
	// Scan from the newest line, so the cap drops the oldest matches.
	for i := len(lines) - 1; i >= 0; i-- {
		found := findAll([]rune(lines[i]), q, fold)
		if len(matches)+len(found) > MaxMatches {
			found = found[len(found)-(MaxMatches-len(matches)):]
			truncated = true
		}
		for k := len(found) - 1; k >= 0; k-- {
			found[k].Line = i
			matches = append(matches, found[k])
		}
		if truncated {
			break
		}
	}
	slices.Reverse(matches)
	return matches, lines, truncated, dropped
}

// plainHistory copies the history's lines without styles. It holds mu only
// for the copy, so a long search never stalls the pane's output.
func (p *Pane) plainHistory() (lines []string, dropped int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sb := p.emu.Scrollback()
	lines = make([]string, 0, sb.Len()+p.emu.Height())
	for i := range sb.Len() {
		lines = append(lines, sb.Line(i).String())
	}
	for _, row := range p.screenLines() {
		lines = append(lines, ansi.Strip(row))
	}
	return lines, p.shift.dropped
}

// findAll returns the matches of q in line, with Line unset. With fold set,
// q is compared to the line in lower case, so q must already be lower case.
func findAll(line, q []rune, fold bool) []Match {
	var out []Match
	for i := 0; i+len(q) <= len(line); {
		if !equalRunes(line[i:i+len(q)], q, fold) {
			i++
			continue
		}
		out = append(out, Match{
			Col: ansi.StringWidth(string(line[:i])),
			Len: ansi.StringWidth(string(line[i : i+len(q)])),
		})
		i += len(q)
	}
	return out
}

func equalRunes(a, q []rune, fold bool) bool {
	for i, r := range a {
		if fold {
			r = unicode.ToLower(r)
		}
		if r != q[i] {
			return false
		}
	}
	return true
}
