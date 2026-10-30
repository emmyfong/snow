// Package convrow formats one past conversation as the cells of a list row.
// The resume picker and an empty chat's list of recent conversations show
// the same row.
package convrow

import (
	"strconv"
	"time"

	"github.com/emmyfong/snow/internal/client/sessionlist"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// Columns returns the cells of it: title, model, the folder when
// withFolder is set, age at now, turns, and "open" while a pane shows it.
// A file that cannot be read has a title that says so. The text comes from
// files the user may not have written, so control characters are dropped.
func Columns(it api.ConversationInfo, now time.Time, withFolder bool) []string {
	name := it.Title
	switch {
	case it.Err != "":
		name = "(cannot read this file)"
	case name == "":
		name = "(no messages yet)"
	}
	cols := []string{name, it.Model}
	if withFolder {
		cols = append(cols, ui.FolderName(it.Folder))
	}
	turns := strconv.Itoa(it.Turns) + " turns"
	if it.Turns == 1 {
		turns = "1 turn"
	}
	open := ""
	if it.Open {
		open = "open"
	}
	cols = append(cols, sessionlist.Ago(max(now.Sub(it.LastActivity), 0)), turns, open)
	for i, c := range cols {
		cols[i] = ui.PlainLine(c)
	}
	return cols
}
