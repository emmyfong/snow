package ui

import "testing"

func TestFolderName(t *testing.T) {
	for in, want := range map[string]string{
		"/home/u/snow":      "snow",
		"/home/u/snow/":     "snow",
		`C:\Users\u\snow`:   "snow",
		`C:\`:               "C:",
		"/":                 "/",
		"":                  "-",
		`\\wsl$\Ubuntu\src`: "src",
		"snow":              "snow",
	} {
		if got := FolderName(in); got != want {
			t.Errorf("FolderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFolderTail(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"/home/a/Projects/snow", "Projects/snow"}, {`C:\work\snow`, "work/snow"}, {"/snow", "snow"}, {"", ""}, {"/", ""}, {"/home/雪/山/", "雪/山"},
	} {
		if got := FolderTail(tt.in); got != tt.want {
			t.Errorf("FolderTail(%q)=%q,want %q", tt.in, got, tt.want)
		}
	}
}

func TestFolderShort(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"/a/b/c", "…/b/c"},
		{"/a/b", "a/b"},
		{`C:\x\y\z`, "…/y/z"},
		{"/", "/"},
		{"", "-"},
	} {
		if got := FolderShort(tt.in); got != tt.want {
			t.Errorf("FolderShort(%q)=%q,want %q", tt.in, got, tt.want)
		}
	}
}
