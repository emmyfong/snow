package convrow

import (
	"slices"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

func TestColumns(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		it     api.ConversationInfo
		folder bool
		want   []string
	}{
		{"one turn", api.ConversationInfo{Title: "fix rename", Model: "a/m", LastActivity: now.Add(-2 * time.Hour), Turns: 1}, false,
			[]string{"fix rename", "a/m", "2h ago", "1 turn", ""}},
		{"many turns, open, with the folder", api.ConversationInfo{Title: "t", Model: "a/m", Folder: "/home/u/api", LastActivity: now, Turns: 3, Open: true}, true,
			[]string{"t", "a/m", "api", "just now", "3 turns", "open"}},
		{"no title", api.ConversationInfo{Model: "a/m", LastActivity: now, Turns: 0}, false,
			[]string{"(no messages yet)", "a/m", "just now", "0 turns", ""}},
		{"unreadable", api.ConversationInfo{Err: "bad line 3", LastActivity: now}, false,
			[]string{"(cannot read this file)", "", "just now", "0 turns", ""}},
		{"escape in a title", api.ConversationInfo{Title: "a\x1b[2Jb", LastActivity: now, Turns: 1}, false,
			[]string{"ab", "", "just now", "1 turn", ""}},
		{"activity in the future", api.ConversationInfo{Title: "t", LastActivity: now.Add(time.Hour), Turns: 1}, false,
			[]string{"t", "", "just now", "1 turn", ""}},
	}
	for _, tt := range tests {
		if got := Columns(tt.it, now, tt.folder); !slices.Equal(got, tt.want) {
			t.Errorf("%s: Columns = %q, want %q", tt.name, got, tt.want)
		}
	}
}
