package store

import (
	"errors"
	"strings"
	"testing"
)

func TestFileName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"api", "api.json"},
		{"web-2_x", "web-2_x.json"},
		{"API", "%41%50%49.json"},
		{"my.app", "my%2Eapp.json"},
		{"a/b", "a%2Fb.json"},
		{`..\x`, "%2E%2E%5Cx.json"},
		{"100%", "100%25.json"},
		{"dev ", "dev%20.json"},
		{"café", "caf%C3%A9.json"},
		{"con", "%63on.json"},
		{"nul", "%6Eul.json"},
		{"com1", "%63om1.json"},
		{"console", "console.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fileName(tt.name)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("fileName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestFileNameRejects(t *testing.T) {
	for _, name := range []string{"", strings.Repeat("x", maxFileName)} {
		if _, err := fileName(name); !errors.Is(err, ErrBadName) {
			t.Fatalf("fileName(%d bytes) error %v, want ErrBadName", len(name), err)
		}
	}
}
