package server

import (
	"testing"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

func TestToTermKey(t *testing.T) {
	tests := []struct {
		in   api.Key
		want term.Key
	}{
		{api.Key{Code: "a", Text: "a"}, term.Key{Code: 'a', Text: "a"}},
		{api.Key{Code: "c", Mod: []string{api.ModCtrl}}, term.Key{Code: 'c', Mod: term.ModCtrl}},
		{api.Key{Code: api.KeyUp}, term.Key{Code: term.KeyUp}},
		{api.Key{Code: api.KeySpace, Text: " "}, term.Key{Code: ' ', Text: " "}},
		{api.Key{Code: api.KeyF5, Mod: []string{api.ModShift, api.ModAlt}}, term.Key{Code: term.KeyF5, Mod: term.ModShift | term.ModAlt}},
		{api.Key{Code: "é", Text: "é"}, term.Key{Code: 'é', Text: "é"}},
	}
	for _, tt := range tests {
		if got := toTermKey(tt.in); got != tt.want {
			t.Errorf("toTermKey(%+v) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}
