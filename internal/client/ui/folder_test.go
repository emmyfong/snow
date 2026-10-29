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
