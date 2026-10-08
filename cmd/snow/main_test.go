package main

import (
	"bytes"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOut  string
		wantCode int
	}{
		{"version", []string{"version"}, "snow dev\n", 0},
		{"no args", nil, "usage: snow version\n", 2},
		{"unknown", []string{"nope"}, "usage: snow version\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			code := run(tt.args, &out)
			if code != tt.wantCode || out.String() != tt.wantOut {
				t.Errorf("run(%q) = %d, %q; want %d, %q", tt.args, code, out.String(), tt.wantCode, tt.wantOut)
			}
		})
	}
}
