package api

import (
	"encoding/json"
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
		{Key{Code: "up", Mod: []string{"hyper"}}, false},
		{Key{}, false},
	}
	for _, tt := range tests {
		if got := tt.key.Valid(); got != tt.want {
			t.Errorf("%+v.Valid() = %v, want %v", tt.key, got, tt.want)
		}
	}
}
