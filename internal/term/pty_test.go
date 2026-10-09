package term

import (
	"slices"
	"testing"
)

func TestWithTerm(t *testing.T) {
	t.Setenv("TERM", "tmux-256color")
	tests := []struct {
		name string
		env  []string
		want string
	}{
		{"inherited env replaces outer TERM", nil, "TERM=xterm-256color"},
		{"explicit env without TERM gets one", []string{"A=1"}, "TERM=xterm-256color"},
		{"explicit TERM is kept", []string{"TERM=dumb"}, "TERM=dumb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withTerm(tt.env)
			var terms []string
			for _, kv := range got {
				if len(kv) >= 5 && kv[:5] == "TERM=" {
					terms = append(terms, kv)
				}
			}
			if !slices.Equal(terms, []string{tt.want}) {
				t.Fatalf("TERM entries %v, want [%s]", terms, tt.want)
			}
		})
	}
}
