package chat

import (
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// folderSnap is a snapshot of pane 2 in folder.
func folderSnap(folder string) *api.ChatSnapshot {
	return &api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "anthropic/claude-sonnet-5-5",
		Mode: "ask", State: api.ChatStateIdle, Folder: folder}
}

// TestFilesDropsAnotherFolder: a late answer for a folder the chat has left
// never fills its links, and the answer for its own folder does.
func TestFilesDropsAnotherFolder(t *testing.T) {
	m, _ := newChat(100, 30)
	m.Snapshot(folderSnap("/work/b"))
	if r := m.FileRequest(); r == nil || r.Pane != 2 {
		t.Fatalf("file request %+v, want one for pane 2", r)
	}
	m.Files(&api.FileList{Pane: 2, Folder: "/work/a", Files: []string{"a.go"}})
	if m.files.loaded || m.known("a.go") {
		t.Fatal("an answer for another folder was taken")
	}
	m.Files(&api.FileList{Pane: 2, Folder: "/work/b", Files: []string{"b.go"}})
	if !m.known("b.go") {
		t.Fatal("the answer for the chat's folder was dropped")
	}
}

// TestFilesRetriesAFailedRead: the chat asks again for a folder whose read
// failed when the next snapshot of the same folder arrives.
func TestFilesRetriesAFailedRead(t *testing.T) {
	m, _ := newChat(100, 30)
	m.Snapshot(folderSnap("/work/project"))
	m.FileRequest()
	m.Files(&api.FileList{Pane: 2, Folder: "/work/project", Err: "permission denied"})
	if m.files.err == "" || m.files.loaded {
		t.Fatalf("after the error: err %q loaded %v", m.files.err, m.files.loaded)
	}
	m.Snapshot(folderSnap("/work/project"))
	if r := m.FileRequest(); r == nil || r.Pane != 2 {
		t.Fatalf("retry request %+v, want one for pane 2", r)
	}
	if m.files.err != "" {
		t.Fatalf("the error %q stayed set for the retry", m.files.err)
	}
}
