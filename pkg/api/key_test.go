package api

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestKeyJSON(t *testing.T) {
	tests := []struct {
		key  Key
		json string
	}{
		{Key{Code: KeyUp}, `{"code":"up"}`},
		{Key{Code: "a", Text: "a"}, `{"code":"a","text":"a"}`},
		{Key{Code: "c", Mod: []string{ModCtrl}}, `{"code":"c","mod":["ctrl"]}`},
		{Key{Code: KeyF5, Mod: []string{ModShift, ModAlt}}, `{"code":"f5","mod":["shift","alt"]}`},
	}
	for _, tt := range tests {
		b, err := json.Marshal(tt.key)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tt.json {
			t.Errorf("Marshal(%+v) = %s, want %s", tt.key, b, tt.json)
		}
	}
}

func TestKeyValid(t *testing.T) {
	tests := []struct {
		key  Key
		want bool
	}{
		{Key{Code: KeyEnter}, true},
		{Key{Code: "a", Text: "a"}, true},
		{Key{Code: "é", Text: "é"}, true},
		{Key{Code: "ab"}, false},
		{Key{Code: " "}, false},            // space is KeySpace
		{Key{Code: "A", Text: "A"}, false}, // Code is the unshifted key: "a" with shift
		{Key{Code: "a", Text: "A", Mod: []string{ModShift}}, true},
		{Key{Code: "1", Text: "1"}, true},
		{Key{Code: KeyUp, Mod: []string{ModCtrl, ModCtrl}}, false}, // Mod is a set
		{Key{Code: "up", Mod: []string{"hyper"}}, false},
		{Key{}, false},
	}
	for _, tt := range tests {
		if got := tt.key.Valid(); got != tt.want {
			t.Errorf("%+v.Valid() = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestParseKey(t *testing.T) {
	tests := []struct {
		name string
		key  Key
	}{
		{"enter", Key{Code: KeyEnter}},
		{"Return", Key{Code: KeyEnter}},
		{"esc", Key{Code: KeyEscape}},
		{"f5", Key{Code: KeyF5}},
		{"space", Key{Code: KeySpace, Text: " "}},
		{"a", Key{Code: "a", Text: "a"}},
		{"A", Key{Code: "a", Text: "A", Mod: []string{ModShift}}},
		{"é", Key{Code: "é", Text: "é"}},
		{"1", Key{Code: "1", Text: "1"}},
		{"!", Key{Code: "!", Text: "!"}},
		{"ctrl+c", Key{Code: "c", Mod: []string{ModCtrl}}},
		{"ctrl+C", Key{Code: "c", Mod: []string{ModCtrl}}},
		{"shift+tab", Key{Code: KeyTab, Mod: []string{ModShift}}},
		{"shift+a", Key{Code: "a", Text: "A", Mod: []string{ModShift}}},
		{"shift+1", Key{Code: "1", Mod: []string{ModShift}}},
		{"ctrl+shift+a", Key{Code: "a", Mod: []string{ModCtrl, ModShift}}},
		{"alt+enter", Key{Code: KeyEnter, Mod: []string{ModAlt}}},
		{"meta+up", Key{Code: KeyUp, Mod: []string{ModMeta}}},
	}
	for _, tt := range tests {
		got, err := ParseKey(tt.name)
		if err != nil {
			t.Errorf("ParseKey(%q): %v", tt.name, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.key) {
			t.Errorf("ParseKey(%q) = %+v, want %+v", tt.name, got, tt.key)
		}
		if !got.Valid() {
			t.Errorf("ParseKey(%q) = %+v, which is not a valid key", tt.name, got)
		}
	}

	bad := []string{"", "ctrl", "ctrl+", "+a", "hyper+a", "ctrl+ctrl+a", "ab", " ", "ctrl+shift"}
	for _, name := range bad {
		if got, err := ParseKey(name); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ParseKey(%q) = %+v, %v; want ErrInvalidKey", name, got, err)
		}
	}
}
