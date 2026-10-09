package term

import "testing"

func TestParseOSC7(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
		ok   bool
	}{
		{"unix path", "file://pc/home/emmy/api", "/home/emmy/api", true},
		{"with command prefix", "7;file://pc/tmp", "/tmp", true},
		{"empty host", "file:///tmp", "/tmp", true},
		{"percent-encoded space", "file://pc/home/emmy/my%20dir", "/home/emmy/my dir", true},
		{"raw space", "file://pc/home/emmy/my dir", "/home/emmy/my dir", true},
		{"windows forward slashes", "file://PC/C:/Users/emmy", `C:\Users\emmy`, true},
		{"windows backslashes from cmd", `file://PC/C:\Users\emmy`, `C:\Users\emmy`, true},
		{"root", "file://pc/", "/", true},
		{"not a file URL", "http://pc/tmp", "", false},
		{"no path", "file://pc", "", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseOSC7([]byte(tt.data))
			if got != tt.want || ok != tt.ok {
				t.Fatalf("parseOSC7(%q) = %q, %v; want %q, %v", tt.data, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPaneFolderFromOSC7(t *testing.T) {
	tests := []struct {
		name, seq, want string
	}{
		{"BEL terminator", "\x1b]7;file://pc/tmp/a\x07", "/tmp/a"},
		{"ST terminator", "\x1b]7;file://pc/tmp/b\x1b\\", "/tmp/b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPane(t)
			f.shellWrites(tt.seq + "marker")
			eventually(t, "marker", func() bool { return p.Text() != "" && contains(p.Text(), "marker") })
			if got := p.Folder(); got != tt.want {
				t.Fatalf("Folder() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPaneFolderKeepsLatest(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("\x1b]7;file://pc/first\x07\x1b]7;file://pc/second\x07done")
	eventually(t, "done", func() bool { return contains(p.Text(), "done") })
	if got := p.Folder(); got != "/second" {
		t.Fatalf("Folder() = %q, want /second", got)
	}
}
