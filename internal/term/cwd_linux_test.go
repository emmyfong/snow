package term

import "testing"

func TestFolderFallsBackToProc(t *testing.T) {
	p := startReal(t, Spec{Path: "/bin/sh", Dir: "/tmp"})
	waitForPrompt(t, p, "$ ")
	if got := p.Folder(); got != "/tmp" {
		t.Fatalf("Folder() = %q, want /tmp from /proc", got)
	}
}
