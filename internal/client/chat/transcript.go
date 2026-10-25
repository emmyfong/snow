package chat

import (
	"fmt"
	"strings"

	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// hit says what a transcript row belongs to, for clicks.
type hit struct {
	tool string // the tool call whose cell holds the row
	box  int    // for a row of the approval box, its first row plus one; else 0
}

// rendered is the transcript drawn at one width. Drawing is the costly
// part of a frame and most frames only turn the spinner, so it is kept
// until something it shows changes.
type rendered struct {
	valid bool
	width int
	rows  []string
	hits  []hit // one per row
}

// invalidate drops the drawn transcript.
func (m *Model) invalidate() { m.drawn.valid = false }

// transcriptRows returns the whole transcript at the pane's width.
func (m *Model) transcriptRows() []string {
	m.draw()
	return m.drawn.rows
}

// draw draws the transcript unless the drawn one is current.
func (m *Model) draw() {
	if m.drawn.valid && m.drawn.width == m.width {
		return
	}
	m.drawn = rendered{valid: true, width: m.width}
	if m.width <= 0 {
		return
	}
	indent := ui.TranscriptIndent(m.width)
	lead := strings.Repeat(" ", indent)
	prevTool := false
	add := func(block string, h hit, tool bool) {
		if block == "" {
			return
		}
		// Tool cells in a row sit together, like a list.
		if len(m.drawn.rows) > 0 && (!tool || !prevTool) {
			m.drawn.rows = append(m.drawn.rows, "")
			m.drawn.hits = append(m.drawn.hits, hit{})
		}
		prevTool = tool
		for _, r := range strings.Split(block, "\n") {
			m.drawn.rows = append(m.drawn.rows, r)
			m.drawn.hits = append(m.drawn.hits, h)
		}
	}
	text := func(kind ui.EntryKind, s string, cut bool) {
		add(ui.Transcript(m.th, ui.TranscriptOptions{Width: m.width, Entries: []ui.TranscriptEntry{{Kind: kind, Text: s, Cut: cut}}}), hit{}, false)
	}
	indented := func(block string) string {
		if indent == 0 || block == "" {
			return block
		}
		return lead + strings.ReplaceAll(block, "\n", "\n"+lead)
	}

	if m.snap.Earlier > 0 {
		text(ui.EntryNote, fmt.Sprintf("%d earlier entries are not shown", m.snap.Earlier), false)
	}
	if len(m.snap.Entries) == 0 && len(m.snap.Queue) == 0 {
		text(ui.EntryNote, "No messages yet. Model: "+m.snap.Model, false)
	}
	for _, e := range m.snap.Entries {
		switch e.Kind {
		case api.EntryUser:
			text(ui.EntryUser, e.Text, false)
		case api.EntryAssistant:
			text(ui.EntryAgent, e.Text, e.Cut)
		case api.EntryThinking:
			text(ui.EntryThinking, e.Text, false)
		case api.EntryTool:
			if e.Tool != nil {
				add(indented(m.toolCell(e.Tool, m.width-indent)), hit{tool: e.Tool.ID}, true)
			}
		case api.EntryModel:
			text(ui.EntryNote, "switched to "+e.Text, false)
		case api.EntryInterrupted:
			text(ui.EntryNote, "interrupted", false)
		case api.EntryError:
			msg := "failed"
			if e.Error != nil {
				msg = e.Error.Message
			}
			text(ui.EntryError, msg, false)
		}
	}
	if p := m.pending(); p != nil {
		top := len(m.drawn.rows)
		if top > 0 {
			top++ // the blank row add puts first
		}
		add(indented(ui.ApprovalBox(m.th, m.boxOptions(p, m.width-indent))), hit{box: top + 1}, false)
	}
	for _, q := range m.snap.Queue {
		text(ui.EntryQueued, "queued: "+q, false)
	}
}

// visible returns the index in the whole transcript of the first row
// shown, and how many rows show.
func (m *Model) visible() (start, n int) {
	all := len(m.transcriptRows())
	end := max(all-m.scroll, 0)
	start = max(end-m.transcriptHeight(), 0)
	return start, end - start
}

// Click handles a left click at x, y in the pane: on a tool cell it
// expands or collapses the cell, and on a choice of the approval box it
// picks that choice. A click anywhere returns to the latest text unless it
// lands in the transcript. It returns what to send, or nil.
func (m *Model) Click(x, y int) *api.ChatInput {
	start, n := m.visible()
	row := y - (m.transcriptHeight() - n)
	if row < 0 || row >= n || y >= m.transcriptHeight() {
		if y >= m.transcriptHeight() {
			m.Follow()
		}
		return nil
	}
	h := m.drawn.hits[start+row]
	switch {
	case h.tool != "":
		m.selected = h.tool
		m.expanded[h.tool] = !m.expanded[h.tool]
		m.invalidate()
	case h.box > 0:
		p := m.pending()
		if p == nil {
			return nil
		}
		o := m.boxOptions(p, m.width-ui.TranscriptIndent(m.width))
		bx, by := x-ui.TranscriptIndent(m.width), start+row-(h.box-1)
		for _, b := range ui.ApprovalButtons(o) {
			if by == b.Y && bx >= b.X && bx < b.X+b.W {
				return m.choose(p, b.Index)
			}
		}
	}
	return nil
}

// toolIDs returns the tool calls in the transcript, oldest first.
func (m *Model) toolIDs() []string {
	var ids []string
	for _, e := range m.snap.Entries {
		if e.Tool != nil {
			ids = append(ids, e.Tool.ID)
		}
	}
	return ids
}

// selectTool moves the selection by step among the tool cells: -1 toward
// older ones. With none selected, it selects the latest.
func (m *Model) selectTool(step int) {
	ids := m.toolIDs()
	if len(ids) == 0 {
		return
	}
	i := len(ids) - 1
	for j, id := range ids {
		if id == m.selected {
			i = min(max(j+step, 0), len(ids)-1)
		}
	}
	m.selected = ids[i]
	m.invalidate()
	m.reveal(ids[i])
}

// reveal scrolls so the first row of tool id's cell shows.
func (m *Model) reveal(id string) {
	m.draw()
	for i, h := range m.drawn.hits {
		if h.tool != id {
			continue
		}
		start, n := m.visible()
		all := len(m.drawn.rows)
		switch {
		case i < start:
			m.scroll = all - i - n
		case i >= start+n:
			m.scroll = max(all-i-1, 0)
		}
		m.Scroll(0)
		return
	}
}
