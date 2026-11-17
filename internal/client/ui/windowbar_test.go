package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestWindowBarParts(t *testing.T) {
	th := theme.Default()
	for _, width := range []int{80, 40, 20} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			row, spans := WindowBarRow(th, WindowBarOptions{Width: width, Session: "work", Tabs: []Tab{{Index: 1, Name: "dev", Active: true}}, NeedsYou: 1})
			if got := ansi.StringWidth(row); got != width {
				t.Fatalf("row width %d, want %d", got, width)
			}
			for _, part := range []string{th.Glyphs.Dropdown, "1 dev*", th.Glyphs.Plus, th.Glyphs.Menu} {
				if !strings.Contains(ansi.Strip(row), part) {
					t.Errorf("row lacks %q: %q", part, ansi.Strip(row))
				}
			}
			for _, span := range spans {
				if span.Kind == TopBarTab && !strings.Contains(ansi.Cut(row, span.X, span.X+span.W), colorCode(th.TabActiveBG)) {
					t.Error("shown tab lacks the active background")
				}
			}
		})
	}
}

func TestWindowBarBadgeHidden(t *testing.T) {
	row := ansi.Strip(WindowBar(theme.Default(), WindowBarOptions{Width: 80, Session: "work", Tabs: []Tab{{Index: 1, Name: "dev", Active: true}}}))
	if strings.Contains(row, "need you") {
		t.Fatalf("empty badge was drawn: %q", row)
	}
}

func TestWindowBarSpans(t *testing.T) {
	for _, width := range []int{80, 40, 20, 8, 1} {
		tabs := make([]Tab, 10)
		for i := range tabs {
			tabs[i] = Tab{Index: i, Name: "a-long-window-name", Active: i == 7}
		}
		o := WindowBarOptions{Width: width, Session: "a-long-session-name", Tabs: tabs, NeedsYou: 2}
		row, spans := WindowBarRow(theme.Default(), o)
		if got := ansi.StringWidth(row); got != width {
			t.Errorf("width %d: got %d cells", width, got)
		}
		plain := ansi.Strip(row)
		for _, span := range spans {
			if span.X < 0 || span.W <= 0 || span.X+span.W > width {
				t.Errorf("width %d: invalid span %+v", width, span)
				continue
			}
			if got := ansi.StringWidth(ansi.Cut(plain, span.X, span.X+span.W)); got != span.W {
				t.Errorf("width %d: span %+v covers %d cells", width, span, got)
			}
		}
		if width >= 20 {
			seen := map[string]bool{}
			for _, span := range spans {
				seen[span.Kind] = true
				if span.Kind == TopBarTab && (span.Index < 0 || span.Index >= len(tabs)) {
					t.Errorf("invalid tab identity %+v", span)
				}
			}
			for _, kind := range []string{TopBarSession, TopBarNewWindow, TopBarMenu} {
				if !seen[kind] {
					t.Errorf("width %d: missing %s in %q", width, kind, plain)
				}
			}
			if seen[TopBarNeedsYou] {
				t.Errorf("width %d: crowded bar kept badge: %q", width, plain)
			}
		}
	}
}

func TestTopBarDrawsWindowFields(t *testing.T) {
	row := ansi.Strip(TopBar(theme.Default(), TopBarOptions{Width: 80, OnWindow: true, Session: "work", Tabs: []Tab{{Index: 1, Name: "dev", Active: true}}, NeedsYou: 1}))
	for _, text := range []string{"work", "1 dev", "+", "need you", "☰"} {
		if !strings.Contains(row, text) {
			t.Errorf("window bar %q does not contain %q", row, text)
		}
	}
}

func TestWindowBarSanitizesUserLabels(t *testing.T) {
	row := ansi.Strip(WindowBar(theme.Default(), WindowBarOptions{
		Width: 80, Session: "work\n\x1b[31mred", Tabs: []Tab{{Index: 1, Name: "dev\n\x1b[2J"}},
	}))
	if strings.ContainsAny(row, "\n\r\x1b") || !strings.Contains(row, "work red") || !strings.Contains(row, "1 dev") {
		t.Fatalf("unsafe labels in single window-bar row: %q", row)
	}
}

func TestWindowBarDropOrder(t *testing.T) {
	th := theme.Default()
	o := WindowBarOptions{Width: 80, Session: "work", Tabs: []Tab{{Index: 4, Name: "development", Active: true}}, NeedsYou: 1}
	full := ansi.Strip(WindowBar(th, o))
	if !strings.Contains(full, "1 need you") {
		t.Fatalf("roomy bar dropped badge: %q", full)
	}
	o.Width = 30
	row, _ := WindowBarRow(th, o)
	got := ansi.Strip(row)
	if strings.Contains(got, "need you") || !strings.Contains(got, "4 development*") {
		t.Fatalf("badge must drop before tab: %q", got)
	}
	o.Width = 20
	row, spans := WindowBarRow(th, o)
	got = ansi.Strip(row)
	if !strings.Contains(got, "…") {
		t.Fatalf("cut tab has no ellipsis: %q", got)
	}
	for _, sp := range spans {
		if sp.Kind == TopBarTab && sp.Index != 4 {
			t.Fatalf("cut tab identity %+v", sp)
		}
	}
}
