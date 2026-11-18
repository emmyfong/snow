package viewer

import (
	"fmt"
	"slices"

	"github.com/emmyfong/snow/internal/textdiff"
)

// Evidence is one recorded change to a file, from the conversation: the
// edit tool's old and new text, or the write tool's new text.
type Evidence struct {
	Turn int
	Tool string
	Old  string
	New  string
}

// Line is one row of the viewer after marks are applied. A removed line has
// Number 0.
type Line struct {
	Number int
	Text   string
	Mark   string
}

// Apply marks the lines that ev still explains. An edit counts only when
// its new text appears exactly once in lines, so text that moved on drops
// its mark instead of pinning the change to the wrong line. Apply returns
// the newest evidence that still explains a mark as attribution, and stale
// when it had evidence but none of it matches.
func Apply(lines []string, ev []Evidence) (out []Line, attribution string, stale bool) {
	added := map[int]bool{}
	removed := map[int][]string{}
	for _, e := range ev {
		add, rem, ok := explain(lines, e)
		if !ok {
			continue
		}
		for _, i := range add {
			added[i] = true
		}
		for i, texts := range rem {
			for _, text := range texts {
				if !slices.Contains(removed[i], text) {
					removed[i] = append(removed[i], text)
				}
			}
		}
		attribution = toolTurn(e)
	}
	for i := 0; i <= len(lines); i++ {
		for _, text := range removed[i] {
			out = append(out, Line{Text: text, Mark: "-"})
		}
		if i == len(lines) {
			break
		}
		mark := ""
		if added[i] {
			mark = "+"
		}
		out = append(out, Line{Number: i + 1, Text: lines[i], Mark: mark})
	}
	if attribution == "" && len(ev) > 0 {
		stale = true
	}
	return out, attribution, stale
}

// explain returns the line indexes e adds and the removed lines with the
// index they go before. It reports false when e cannot be placed.
func explain(lines []string, e Evidence) (added []int, removed map[int][]string, ok bool) {
	newLines := textdiff.Split(e.New)
	if len(newLines) == 0 {
		return nil, nil, false
	}
	start, found := find(lines, newLines)
	if found != 1 {
		return nil, nil, false
	}
	removed = map[int][]string{}
	pos := start
	for _, d := range textdiff.Lines(e.Old, e.New) {
		switch d.Op {
		case textdiff.Keep:
			pos++
		case textdiff.Add:
			if pos >= start+len(newLines) {
				return nil, nil, false
			}
			added = append(added, pos)
			pos++
		case textdiff.Remove:
			removed[pos] = append(removed[pos], d.Text)
		}
	}
	if pos != start+len(newLines) {
		return nil, nil, false
	}
	return added, removed, true
}

// find returns how many times block appears in lines, up to two, and the
// first start.
func find(lines, block []string) (start, count int) {
	start = -1
	for i := 0; i+len(block) <= len(lines); i++ {
		if slices.Equal(lines[i:i+len(block)], block) {
			count++
			if start < 0 {
				start = i
			}
			if count == 2 {
				return start, count
			}
		}
	}
	return start, count
}

// toolTurn names e for the header.
func toolTurn(e Evidence) string {
	if e.Turn <= 0 {
		return e.Tool
	}
	return fmt.Sprintf("%s, turn %d", e.Tool, e.Turn)
}
