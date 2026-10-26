package convstore

import "testing"

// TestConversationTitleMatchesList: the title a loaded conversation gives
// is the one the history list shows: the last rename, else the meta title,
// else the start of the first message.
func TestConversationTitleMatchesList(t *testing.T) {
	long := "explain the rename command and then every place that calls it, in the whole project"
	for _, tt := range []struct {
		name  string
		title string // in the meta record
		recs  []Record
		want  string
	}{
		{"first message", "", []Record{&User{At: at, Text: "fix the bug"}}, "fix the bug"},
		{"long first message", "", []Record{&User{At: at, Text: long}}, titleFrom(long)},
		{"meta title", "From meta", []Record{&User{At: at, Text: "fix the bug"}}, "From meta"},
		{"renamed", "From meta", []Record{&User{At: at, Text: "fix the bug"}, &Rename{At: at, Title: "One"}, &Rename{At: at, Title: "Two"}}, "Two"},
		{"nothing", "", nil, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := openStore(t)
			m := *testMeta()
			m.Title = tt.title
			w, err := s.Create(m)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = w.Close() }()
			for _, r := range tt.recs {
				if err := w.Append(r); err != nil {
					t.Fatal(err)
				}
			}
			c, err := s.Load(w.ID())
			if err != nil {
				t.Fatal(err)
			}
			if got := c.Title(); got != tt.want {
				t.Fatalf("Title() = %q, want %q", got, tt.want)
			}
			entries, err := s.ListAll()
			if err != nil || len(entries) != 1 || entries[0].Title != c.Title() {
				t.Fatalf("list title %q, loaded title %q, %v", entries[0].Title, c.Title(), err)
			}
		})
	}
}

// TestOneLine: control characters become spaces and runs of whitespace
// collapse, so a title never carries an escape sequence to a screen.
func TestOneLine(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"  Fix the\trename\x1b[2J bug \n", "Fix the rename [2J bug"},
		{"hi \x1b]52;c;AAAA\x07 there", "hi ]52;c;AAAA there"},
		{"\x00\x01", ""},
		{"plain", "plain"},
	} {
		if got := OneLine(tt.in); got != tt.want {
			t.Errorf("OneLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestFirstMessageTitleHasNoControlCharacters: the list title made from a
// first message is clean even when the message holds an escape sequence.
func TestFirstMessageTitleHasNoControlCharacters(t *testing.T) {
	if got := titleFrom("hi \x1b]52;c;AAAA\x07 there"); got != "hi ]52;c;AAAA there" {
		t.Fatalf("titleFrom = %q", got)
	}
}
