package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func sample(name string) Snapshot {
	return Snapshot{
		Name:   name,
		Active: 1,
		Windows: []Window{
			{
				Name:    "dev",
				Layout:  json.RawMessage(`{"split":"v","children":[1,2]}`),
				Focused: 2,
				Panes: []Pane{
					{ID: 1, Profile: "Ubuntu", Folder: "/home/emmy/api", Scrollback: []string{"$ ls", "\x1b[34mmain.go\x1b[m"}},
					{ID: 2, Profile: "PowerShell", Folder: `C:\Projects\snow`},
				},
			},
			{Name: "logs", Panes: []Pane{{ID: 3, Profile: "bash"}}},
		},
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	in := sample("api")
	in.Saved = time.Date(2026, 10, 9, 9, 14, 0, 0, time.UTC)
	b, err := encode(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decode(b)
	if err != nil {
		t.Fatal(err)
	}
	in.Version = Version
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip changed the snapshot:\n got %#v\nwant %#v", out, in)
	}
}

func TestEncodeKeepsLastScrollbackLines(t *testing.T) {
	lines := make([]string, MaxScrollback+5)
	for i := range lines {
		lines[i] = fmt.Sprint(i)
	}
	in := Snapshot{Name: "api", Windows: []Window{{Panes: []Pane{{ID: 1, Scrollback: lines}}}}}
	b, err := encode(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decode(b)
	if err != nil {
		t.Fatal(err)
	}
	got := out.Windows[0].Panes[0].Scrollback
	if len(got) != MaxScrollback || got[0] != "5" || got[len(got)-1] != fmt.Sprint(MaxScrollback+4) {
		t.Fatalf("kept %d lines from %q to %q, want the last %d", len(got), got[0], got[len(got)-1], MaxScrollback)
	}
	if len(in.Windows[0].Panes[0].Scrollback) != MaxScrollback+5 {
		t.Fatal("encode changed the caller's scrollback")
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"not json", `{"version":1,`},
		{"wrong type", `{"version":"one"}`},
		{"no version", `{"name":"api"}`},
		{"no name", `{"version":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decode([]byte(tt.data))
			var de *DecodeError
			if !errors.As(err, &de) {
				t.Fatalf("decode(%s) error %v, want a *DecodeError", tt.data, err)
			}
		})
	}
}

// TestSnapshotChatPaneRoundTrip: a chat pane's kind and conversation ID
// survive save and load. A chat saves no scrollback: its conversation file
// is the source of truth.
func TestSnapshotChatPaneRoundTrip(t *testing.T) {
	in := Snapshot{Name: "work", Windows: []Window{{
		Index: 1, Name: "chat", Focused: 1,
		Panes: []Pane{
			{ID: 1, Kind: KindChat, Profile: "bash", Folder: "/home/emmy/api", Conversation: "20261029-220201-f91d9d0d"},
			{ID: 2, Profile: "bash", Folder: "/home/emmy/api", Scrollback: []string{"$ ls"}},
		},
	}}}
	b, err := encode(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decode(b)
	if err != nil {
		t.Fatal(err)
	}
	in.Version = Version
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip changed the snapshot:\n got %#v\nwant %#v", out, in)
	}
}

// TestDecodeFormatOne: a file the previous format wrote has no pane kinds
// and still loads. Its panes are terminals.
func TestDecodeFormatOne(t *testing.T) {
	old := `{"version":1,"name":"api","saved":"2026-10-09T09:14:00Z","active":1,` +
		`"windows":[{"index":1,"name":"dev","panes":[{"id":1,"profile":"bash","folder":"/home/emmy"}]}]}`
	s, err := decode([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if p := s.Windows[0].Panes[0]; p.Kind != "" || p.Conversation != "" || p.Profile != "bash" {
		t.Fatalf("pane %+v, want a plain terminal pane", p)
	}
}

// TestDecodeNewerFormat: a file in a format newer than this Snow reads is
// refused with ErrNewerVersion, not read as far as it goes.
func TestDecodeNewerFormat(t *testing.T) {
	raw := fmt.Sprintf(`{"version":%d,"name":"api","windows":[]}`, Version+1)
	if _, err := decode([]byte(raw)); !errors.Is(err, ErrNewerVersion) {
		t.Fatalf("decode error %v, want ErrNewerVersion", err)
	}
}
