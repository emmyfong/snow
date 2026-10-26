package whichkey

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/theme"
)

func rows(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

func TestViewListsKeysAndDescriptions(t *testing.T) {
	tab := keys.Default()
	box := View(theme.Default(), Options{Prefix: tab.PrefixLabel(), Entries: tab.Entries(), MaxWidth: 76, MaxHeight: 22})
	text := ansi.Strip(box)
	for _, want := range []string{"Ctrl+b", "%", "Split right", "0-9", "Select window", "Ctrl+Arrows", "Detach"} {
		if !strings.Contains(text, want) {
			t.Errorf("popup lacks %q:\n%s", want, text)
		}
	}
}

func TestViewNeverExceedsItsBounds(t *testing.T) {
	tab := keys.Default()
	for _, size := range [][2]int{{76, 22}, {30, 22}, {80, 8}, {20, 5}, {12, 3}, {4, 4}, {3, 3}, {0, 0}} {
		box := View(theme.Default(), Options{Prefix: "Ctrl+b", Entries: tab.Entries(), MaxWidth: size[0], MaxHeight: size[1]})
		if box == "" {
			continue
		}
		rs := rows(box)
		if len(rs) > size[1] {
			t.Errorf("max %v: %d rows", size, len(rs))
		}
		for _, r := range rs {
			if ansi.StringWidth(r) > size[0] {
				t.Errorf("max %v: row %q is %d wide", size, r, ansi.StringWidth(r))
			}
		}
	}
}

func TestViewUsesTwoColumnsWhenShort(t *testing.T) {
	tab := keys.Default()
	es := tab.Entries()
	tall := rows(View(theme.Default(), Options{Prefix: "Ctrl+b", Entries: es, MaxWidth: 100, MaxHeight: 40}))
	short := rows(View(theme.Default(), Options{Prefix: "Ctrl+b", Entries: es, MaxWidth: 100, MaxHeight: 13}))
	if len(tall) != len(es)+2 {
		t.Errorf("tall popup has %d rows, want one per entry plus the frame (%d)", len(tall), len(es)+2)
	}
	if len(short) > 13 || len(short) >= len(tall) {
		t.Errorf("short popup has %d rows, want fewer than %d and at most 13", len(short), len(tall))
	}
	text := strings.Join(short, "\n")
	for _, e := range es {
		if !strings.Contains(text, e.Desc) {
			t.Errorf("two-column popup dropped %q", e.Desc)
		}
	}
}

func TestViewEmpty(t *testing.T) {
	if got := View(theme.Default(), Options{Prefix: "Ctrl+b", MaxWidth: 80, MaxHeight: 24}); got != "" {
		t.Errorf("no entries drew %q", got)
	}
}
