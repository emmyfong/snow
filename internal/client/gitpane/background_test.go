package gitpane

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func backgroundPane(t *testing.T, profile colorprofile.Profile, painted bool, width, height int) (view, background string) {
	t.Helper()
	base, ok := theme.ByName("Catppuccin Mocha")
	if !ok {
		t.Fatal("missing built-in theme")
	}
	var known color.Color
	if painted {
		known = base.Background
	}
	th := base.ForBackground(profile, known)
	m := model(api.GitState{Folder: "/home/u/snow", Branch: "main"}, width, height)
	m.SetTheme(th)
	m.Key(key("s"))
	var buf bytes.Buffer
	w := colorprofile.Writer{Forward: &buf, Profile: profile}
	if _, err := w.WriteString(strings.Join(m.View(), "\n")); err != nil {
		t.Fatal(err)
	}
	background = "#ffffff"
	if painted {
		r, g, b, _ := th.Background.RGBA()
		background = fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	}
	return buf.String(), background
}

func TestGitPaneBackgroundGolden(t *testing.T) {
	for _, painted := range []bool{false, true} {
		mode := "terminal"
		if painted {
			mode = "painted"
		}
		for _, profile := range []colorprofile.Profile{colorprofile.ANSI, colorprofile.ANSI256} {
			for _, size := range [][2]int{{80, 24}, {120, 40}} {
				t.Run(fmt.Sprintf("%s_%s_%dx%d", mode, profile, size[0], size[1]), func(t *testing.T) {
					view, _ := backgroundPane(t, profile, painted, size[0], size[1])
					golden.RequireEqual(t, view)
				})
			}
		}
	}
}
