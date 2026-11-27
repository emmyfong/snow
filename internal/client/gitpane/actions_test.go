package gitpane

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func actionModel(kind string, width, height int) *Model {
	var m *Model
	switch kind {
	case "staged":
		m = model(changes(), width, height)
	case "unstaged":
		m = model(api.GitState{Changes: []api.GitChange{{Path: "README.md", Work: "M"}}}, width, height)
	case "worktree":
		m = worktreeModel(width, height)
		m.Key(key("down"))
	case "history":
		m = model(history(), width, height)
	default:
		m = model(api.GitState{Branch: "main"}, width, height)
	}
	m.SetFocused(true)
	return m
}

func TestListButtons(t *testing.T) {
	for kind, labels := range map[string][]string{"staged": {"Unstage", "Diff"}, "unstaged": {"Stage", "Diff"}, "worktree": {"Open"}, "history": {}, "empty": {"Refresh"}} {
		m := actionModel(kind, 80, 24)
		row := ansi.Strip(m.View()[23])
		for _, label := range append(labels, "Commit...") {
			if !strings.Contains(row, label) {
				t.Errorf("%s row lacks %q: %q", kind, label, row)
			}
		}
		for _, label := range []string{"Stage", "Unstage", "Diff", "Open", "Refresh"} {
			if strings.Contains(row, label) && !containsLabel(labels, label) {
				t.Errorf("%s row offers %q: %q", kind, label, row)
			}
		}
		m.SetFocused(false)
		if row := ansi.Strip(m.View()[23]); strings.TrimSpace(row) != "" {
			t.Errorf("unfocused %s: %q", kind, row)
		}
		for _, h := range m.layout().hits {
			if h == hitButtons {
				t.Fatalf("unfocused %s has buttons", kind)
			}
		}
	}
}

func containsLabel(labels []string, label string) bool {
	for _, s := range labels {
		if s == label {
			return true
		}
	}
	return false
}

func TestButtonClickSendsKey(t *testing.T) {
	for kind, names := range map[string][]string{"staged": {"u", "enter", "c"}, "unstaged": {"s", "enter", "c"}, "worktree": {"enter", "c"}, "history": {"c"}, "empty": {"r", "c"}} {
		for _, name := range names {
			click, press := actionModel(kind, 80, 24), actionModel(kind, 80, 24)
			label := map[string]string{"u": "Unstage", "s": "Stage", "enter": "Diff", "c": "Commit...", "r": "Refresh"}[name]
			if kind == "worktree" && name == "enter" {
				label = "Open"
			}
			row := ansi.Strip(click.View()[23])
			x := strings.Index(row, label)
			if x < 0 {
				t.Fatalf("%s lacks %s: %q", kind, label, row)
			}
			got, want := click.Click(x, 23), press.Key(key(name))
			if !reflect.DeepEqual(got, want) || click.mode != press.mode || click.message() != press.message() || click.committing != press.committing {
				t.Errorf("%s %s click %#v mode %v, key %#v mode %v", kind, label, got, click.mode, want, press.mode)
			}
		}
	}
}

func TestButtonRowNarrow(t *testing.T) {
	for _, width := range []int{22, 30, 80} {
		m := actionModel("unstaged", width, 8)
		row := ansi.Strip(m.View()[7])
		if !strings.Contains(row, "Commit...") || ansi.StringWidth(row) != width {
			t.Errorf("width %d row %q", width, row)
		}
		if width == 22 && strings.Contains(row, "Diff") {
			t.Errorf("22 columns keep Diff: %q", row)
		}
		for _, sp := range ui.ButtonRowSpans(ui.ButtonRowOptions{Width: width, Choices: []string{"Stage", "Diff"}, Right: "Commit..."}) {
			if sp.X+sp.W > width {
				t.Fatalf("span outside width: %+v", sp)
			}
		}
	}
}

func TestButtonRowHidesWhenShort(t *testing.T) {
	for _, height := range []int{3, 4} {
		m := actionModel("unstaged", 80, height)
		row := ansi.Strip(m.View()[height-1])
		if strings.Contains(row, "Commit...") != (height == 4) {
			t.Errorf("height %d row %q", height, row)
		}
	}
}

func TestButtonRowReservesSpaceForWrappedReadError(t *testing.T) {
	m := model(api.GitState{Error: strings.Repeat("git read error ", 30)}, 30, 8)
	row := ansi.Strip(m.View()[7])
	if !strings.Contains(row, "Refresh") || !strings.Contains(row, "Commit...") {
		t.Fatalf("wrapped read error displaced buttons: %q", row)
	}
}
