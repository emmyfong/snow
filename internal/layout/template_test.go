package layout

import (
	"errors"
	"reflect"
	"testing"
)

// ide is files | (viewer / chat / terminal) | git.
var ide = Template{Axis: Horizontal, Ratios: []float64{0.2, 0.6, 0.2}, Children: []Template{
	{Kind: "files"},
	{Axis: Vertical, Ratios: []float64{0.4, 0.4, 0.2}, Children: []Template{{Kind: "viewer"}, {Kind: "chat"}, {Kind: "terminal"}}},
	{Kind: "git"},
}}

func TestTemplateBuild(t *testing.T) {
	onlyChatTerm, _ := ide.Prune(func(k string) bool { return k == "chat" || k == "terminal" })
	tests := []struct {
		name       string
		tmpl       Template
		cols, rows int
		want       map[PaneID]Rect
	}{
		{"leaf 80x24", Template{Kind: "chat"}, 80, 24, map[PaneID]Rect{1: {W: 80, H: 24}}},
		{"ide 80x24", ide, 80, 24, map[PaneID]Rect{
			1: {W: 16, H: 24}, 2: {X: 17, W: 46, H: 9}, 3: {X: 17, Y: 10, W: 46, H: 9}, 4: {X: 17, Y: 20, W: 46, H: 4}, 5: {X: 64, W: 16, H: 24},
		}},
		{"ide 120x40", ide, 120, 40, map[PaneID]Rect{
			1: {W: 24, H: 40}, 2: {X: 25, W: 70, H: 15}, 3: {X: 25, Y: 16, W: 70, H: 15}, 4: {X: 25, Y: 32, W: 70, H: 8}, 5: {X: 96, W: 24, H: 40},
		}},
		{"ide pruned to chat and terminal 80x24", onlyChatTerm, 80, 24, map[PaneID]Rect{1: {W: 80, H: 15}, 2: {Y: 16, W: 80, H: 8}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := make([]PaneID, len(tt.tmpl.Kinds()))
			for i := range ids {
				ids[i] = PaneID(i + 1)
			}
			tree, err := tt.tmpl.Build(ids, tt.cols, tt.rows)
			if err != nil {
				t.Fatal(err)
			}
			if got := Rects(tree, tt.cols, tt.rows).Panes; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("rects\n got %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestTemplateTooSmall(t *testing.T) {
	tree := New(7)
	before := *tree.clone()
	err := tree.Apply(ide, []PaneID{1, 2, 3, 4, 5}, 6, 3)
	if !errors.Is(err, ErrTooSmall) {
		t.Fatalf("err %v, want ErrTooSmall", err)
	}
	if !reflect.DeepEqual(*tree, before) {
		t.Fatalf("tree changed to %+v", tree)
	}
}

func TestTemplatePrune(t *testing.T) {
	tests := []struct {
		name  string
		keep  []string
		kinds []string
		ok    bool
	}{
		{"all", []string{"files", "viewer", "chat", "terminal", "git"}, []string{"files", "viewer", "chat", "terminal", "git"}, true},
		{"chat only", []string{"chat"}, []string{"chat"}, true},
		{"none", nil, nil, false},
		{"files and git", []string{"files", "git"}, []string{"files", "git"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keep := map[string]bool{}
			for _, k := range tt.keep {
				keep[k] = true
			}
			got, ok := ide.Prune(func(k string) bool { return keep[k] })
			if ok != tt.ok {
				t.Fatalf("ok %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			if k := got.Kinds(); !reflect.DeepEqual(k, tt.kinds) {
				t.Fatalf("kinds %v, want %v", k, tt.kinds)
			}
		})
	}
}

func TestTemplateValidate(t *testing.T) {
	tests := []struct {
		name string
		tmpl Template
		ok   bool
	}{
		{"leaf", Template{Kind: "chat"}, true},
		{"empty leaf", Template{}, false},
		{"leaf with axis", Template{Kind: "chat", Axis: Horizontal}, false},
		{"one child", Template{Axis: Horizontal, Ratios: []float64{1}, Children: []Template{{Kind: "a"}}}, false},
		{"bad sum", Template{Axis: Horizontal, Ratios: []float64{0.5, 0.6}, Children: []Template{{Kind: "a"}, {Kind: "b"}}}, false},
		{"split with kind", Template{Kind: "x", Axis: Horizontal, Ratios: []float64{0.5, 0.5}, Children: []Template{{Kind: "a"}, {Kind: "b"}}}, false},
		{"ide", ide, true},
	}
	for _, tt := range tests {
		if err := tt.tmpl.Validate(); (err == nil) != tt.ok {
			t.Errorf("%s: err %v, want ok %v", tt.name, err, tt.ok)
		}
	}
}

func TestTemplateBuildWrongCount(t *testing.T) {
	if _, err := ide.Build([]PaneID{1, 2}, 80, 24); !errors.Is(err, ErrInvalidTree) {
		t.Fatalf("err %v, want ErrInvalidTree", err)
	}
}
