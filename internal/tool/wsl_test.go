package tool

import (
	"context"
	"testing"
)

// TestRealPathUsesApprovedTarget: the tool opens the target approval
// resolved, so a link that changes after approval cannot redirect it.
func TestRealPathUsesApprovedTarget(t *testing.T) {
	env := Env{
		Folder:   Folder{Path: "/folder", Distro: "Ubuntu"},
		Approved: map[string]string{"/folder/link": "/outside/target"},
	}
	got, err := env.RealPath(context.Background(), Path{Logical: "/folder/link", OS: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	if want := wslOSPath("Ubuntu", "/outside/target"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestRealPathRejectsUnresolvedApproval: a path approval could not resolve
// fails closed instead of opening a new target.
func TestRealPathRejectsUnresolvedApproval(t *testing.T) {
	env := Env{
		Folder:   Folder{Path: "/folder", Distro: "Ubuntu"},
		Approved: map[string]string{"/folder/link": ""},
	}
	if _, err := env.RealPath(context.Background(), Path{Logical: "/folder/link"}); err == nil {
		t.Fatal("an unresolved approval opened a path")
	}
}

// TestRealPathLocalFolder: a folder of the OS Snow runs on opens the path as
// it is, with no approval record.
func TestRealPathLocalFolder(t *testing.T) {
	env := Env{Folder: Folder{Path: "/folder"}}
	got, err := env.RealPath(context.Background(), Path{Logical: "/folder/a", OS: "/folder/a"})
	if err != nil || got != "/folder/a" {
		t.Fatalf("got %q, %v", got, err)
	}
}
