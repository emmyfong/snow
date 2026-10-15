package term

import (
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
// query has an upper-case letter. Truncated is true when there were more than
// MaxMatches. An empty query matches nothing.
func (p *Pane) Search(query string) (matches []Match, truncated bool) {
	q := []rune(query)
	if len(q) == 0 {
		return nil, false
	}
	fold := true
	for _, r := range q {
		if unicode.IsUpper(r) {
			fold = false
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	sb := p.emu.Scrollback()
	kept := sb.Len()
	lines := make([]string, 0, kept+p.emu.Height())
	for i := range kept {
		lines = append(lines, sb.Line(i).String())
	}
	for _, row := range p.screenLines() {
		lines = append(lines, ansi.Strip(row))
	}
	for i, line := range lines {
		for _, m := range findAll([]rune(line), q, fold) {
			if len(matches) == MaxMatches {
				return matches, true
			}
			m.Line = i
			matches = append(matches, m)
		}
	}
	return matches, false
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
