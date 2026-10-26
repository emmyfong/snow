package command

import (
	"testing"
)

// TestListGroups: a grouped list sorts commands into their sections and
// puts one heading before each, while a flat list keeps its order.
func TestListGroups(t *testing.T) {
	cmds := []Command{
		{Name: "help", Desc: "Show help", Group: GroupSnow},
		{Name: "model", Args: "[id]", Desc: "Pick a model", Group: GroupModel},
		{Name: "new", Desc: "New conversation", Group: GroupConversation},
	}
	l := NewList(cmds, true, true)
	if got, want := commandNames(l.Commands), []string{"model", "new", "help"}; !equal(got, want) {
		t.Fatalf("commands %v, want %v", got, want)
	}
	var heads []string
	var rows []string
	for i, it := range l.Items {
		if it.Heading != "" {
			heads = append(heads, it.Heading)
			if l.ItemOf[i] != -1 {
				t.Errorf("heading %q names command %d", it.Heading, l.ItemOf[i])
			}
			continue
		}
		rows = append(rows, it.Cols[0])
	}
	if want := []string{"Model", "Conversation", "Snow"}; !equal(heads, want) {
		t.Errorf("headings %v, want %v", heads, want)
	}
	if want := []string{"/model [id]", "/new", "/help"}; !equal(rows, want) {
		t.Errorf("rows %v, want %v", rows, want)
	}
	if got := l.Selected(1); got != 3 {
		t.Errorf("Selected(1) = %d, want the row after the Conversation heading", got)
	}

	flat := NewList(cmds, false, false)
	if len(flat.Items) != len(cmds) {
		t.Fatalf("flat list has %d rows, want %d", len(flat.Items), len(cmds))
	}
	for i, it := range flat.Items {
		if it.Heading != "" || len(it.Cols) != 1 || flat.ItemOf[i] != i {
			t.Errorf("flat row %d = %+v (command %d)", i, it, flat.ItemOf[i])
		}
	}
}

func commandNames(cs []Command) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
