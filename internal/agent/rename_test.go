package agent

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
)

// TestRenameAppendsRecord: a rename adds one record at the end of the
// file, reports the new title, and leaves every earlier byte as it was.
func TestRenameAppendsRecord(t *testing.T) {
	h := newHarness(t, nil, text("ok"))
	_ = h.conv.Send("hi")
	next[*TurnEnded](t, h)
	path := h.conv.w.Path()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.conv.Rename("  Fix the\trename\x1b[2J bug \n"); err != nil {
		t.Fatal(err)
	}
	if e := next[*TitleChanged](t, h); e.Title != "Fix the rename [2J bug" || e.Failure != nil {
		t.Fatalf("title changed %+v", e)
	}
	_ = h.conv.Close()
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, before) {
		t.Fatal("a rename changed earlier lines of the file")
	}
	recs := h.records(t)
	last, ok := recs[len(recs)-1].(*convstore.Rename)
	if !ok || last.Title != "Fix the rename [2J bug" {
		t.Fatalf("last record %#v", recs[len(recs)-1])
	}
}

// TestRenameRefusesBadTitles: a blank title and a long one are refused
// before anything is written.
func TestRenameRefusesBadTitles(t *testing.T) {
	h := newHarness(t, nil)
	for name, title := range map[string]string{
		"empty":    "",
		"blank":    " \t\n ",
		"too long": strings.Repeat("x", MaxTitle+1),
	} {
		if err := h.conv.Rename(title); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := h.conv.Rename(strings.Repeat("é", MaxTitle)); err != nil {
		t.Fatalf("a title of MaxTitle characters: %v", err)
	}
	next[*TitleChanged](t, h)
	_ = h.conv.Close()
	if err := h.conv.Rename("late"); !errors.Is(err, ErrClosed) {
		t.Fatalf("rename after close: %v", err)
	}
}

// TestStateOfLoaded: the model, effort, and mode of a loaded file are the
// last ones saved in it, and ResumeLoaded goes on with them.
func TestStateOfLoaded(t *testing.T) {
	h := newHarness(t, nil)
	_ = h.conv.SwitchModel("fake/model-2", "high")
	next[*ModelChanged](t, h)
	_ = h.conv.SetMode(ModeAuto)
	next[*ModeChanged](t, h)
	id := h.conv.ID()
	_ = h.conv.Close()
	loaded, err := h.store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if model, effort, mode := StateOf(loaded); model != "fake/model-2" || effort != "high" || mode != ModeAuto {
		t.Fatalf("StateOf = %q, %q, %q", model, effort, mode)
	}
	c, _, err := ResumeLoaded(h.config(t), loaded)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if c.Mode() != ModeAuto || c.ID() != id {
		t.Fatalf("resumed %s in mode %s", c.ID(), c.Mode())
	}
}
