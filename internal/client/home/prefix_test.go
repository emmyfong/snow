package home

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/config"
)

// prefixGolden presses Ctrl+b on the home page, which has no prefix keys.
func prefixGolden(t *testing.T, cols, rows int) {
	m := screen(cols, rows, session("api", time.Minute, 1), session("web", time.Hour, 0))
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'b', Mod: tea.ModCtrl}))
	if !strings.Contains(m.tip, "Ctrl+b keys work inside a session") || m.notice != "" {
		t.Fatalf("tip %q, notice %q, want where Ctrl+b keys work as a tip", m.tip, m.notice)
	}
	golden.RequireEqual(t, m.View().Content)
}

// TestPrefixNoticeGolden80x24: Ctrl+b on the home page says the prefix
// keys work inside a session, instead of doing nothing (#287).
func TestPrefixNoticeGolden80x24(t *testing.T)  { prefixGolden(t, 80, 24) }
func TestPrefixNoticeGolden120x40(t *testing.T) { prefixGolden(t, 120, 40) }

// TestReboundPrefixOnHome: with the prefix moved to Ctrl+a, the tip and the
// help name Ctrl+a, and Ctrl+b is an ordinary key.
func TestReboundPrefixOnHome(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Prefix = "C-a"
	tab, err := keys.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	m := screen(80, 24)
	m.SetKeys(tab)
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'b', Mod: tea.ModCtrl}))
	if m.tip != "" {
		t.Fatalf("Ctrl+b gave tip %q, want none: it is not the prefix", m.tip)
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	if !strings.HasPrefix(m.tip, "Ctrl+a keys") {
		t.Fatalf("tip %q, want it to name Ctrl+a", m.tip)
	}
	for _, k := range m.helpKeys() {
		if strings.Contains(k.Keys, "Ctrl+b") {
			t.Fatalf("help lists %q, want the rebound prefix", k.Keys)
		}
	}
}
