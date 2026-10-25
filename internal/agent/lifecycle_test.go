package agent

import (
	"errors"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/tool"
)

// TestStartSavesProfile: the meta record names the shell profile.
func TestStartSavesProfile(t *testing.T) {
	h := newHarness(t, nil)
	cfg := h.config(t)
	cfg.Shell = shell.Profile{Name: "Ubuntu", Kind: shell.KindWSL}
	c, err := Start(cfg, convstore.Meta{Model: testModel})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	loaded, err := h.store.Load(c.ID())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Meta.Profile != "Ubuntu" || loaded.Meta.Folder != h.folder {
		t.Fatalf("meta %+v", loaded.Meta)
	}
}

// TestResumeRefusesOtherFolder: a resume in another folder is refused, so
// tools never work where the conversation did not.
func TestResumeRefusesOtherFolder(t *testing.T) {
	h := newHarness(t, nil)
	id := h.conv.ID()
	_ = h.conv.Close()
	cfg := h.config(t)
	cfg.Folder = tool.Folder{Path: t.TempDir()}
	if _, _, err := Resume(cfg, id); !errors.Is(err, ErrFolderMismatch) {
		t.Fatalf("resume elsewhere: %v", err)
	}
	c, _, err := Resume(h.config(t), id)
	if err != nil {
		t.Fatalf("resume in place: %v", err)
	}
	_ = c.Close()
}

// TestTypedErrors: callers can match a closed conversation and a bad
// argument.
func TestTypedErrors(t *testing.T) {
	h := newHarness(t, nil)
	for name, err := range map[string]error{
		"mode":     h.conv.SetMode("yolo"),
		"decision": h.conv.Answer(Answer{ID: "x", Decision: "maybe"}),
		"model":    h.conv.SwitchModel("no-slash", ""),
	} {
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	_ = h.conv.Close()
	if err := h.conv.Send("hi"); !errors.Is(err, ErrClosed) {
		t.Fatalf("send after close: %v", err)
	}
}

// TestCloseFinishesSaves: a mode change right before Close is in the file.
func TestCloseFinishesSaves(t *testing.T) {
	h := newHarness(t, nil)
	_ = h.conv.SetMode(ModeAuto)
	if err := h.conv.Close(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range h.records(t) {
		if m, ok := r.(*convstore.Mode); ok && m.Mode == string(ModeAuto) {
			found = true
		}
	}
	if !found {
		t.Fatal("the mode change was lost")
	}
}
