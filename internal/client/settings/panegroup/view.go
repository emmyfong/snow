package panegroup

import (
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

func groupView(th theme.Theme, width, height int, label, value, scope, key string) string {
	items := []ui.ListItem{
		{Heading: label},
		{Cols: []string{value}},
		{Heading: "Effective scope"},
		{Cols: []string{scope}},
		{Heading: "Change in"},
		{Cols: []string{"~/.snow/config.toml"}},
		{Heading: "Key"},
		{Cols: []string{key}},
		{Heading: "No runtime reports these controls yet."},
	}
	return ui.List(th, ui.ListOptions{Items: items, Selected: -1, Width: width, Height: height})
}
