package term

import "testing"

func TestWithTerm(t *testing.T) {
	t.Setenv("TERM", "tmux-256color")
	tests := []struct {
		name string
		env  []string
		want string
	}{
		{"outer TERM is replaced", nil, "TERM=xterm-256color"},
		{"additions without TERM keep the pane TERM", []string{"A=1"}, "TERM=xterm-256color"},
		{"an added TERM wins", []string{"TERM=dumb"}, "TERM=dumb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withTerm(tt.env)
			// exec.Cmd keeps the last value of a duplicated key.
			last := ""
			for _, kv := range got {
				if len(kv) >= 5 && kv[:5] == "TERM=" {
					last = kv
				}
			}
			if last != tt.want {
				t.Fatalf("effective TERM %q, want %q", last, tt.want)
			}
		})
	}
}
