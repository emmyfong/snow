package main

import (
	"errors"
	"testing"

	"github.com/emmyfong/snow/internal/client"
)

func TestLeaveLine(t *testing.T) {
	tests := []struct {
		name string
		res  client.Result
		want string
	}{
		{name: "none"},
		{name: "one", res: client.Result{Running: 1}, want: "Snow is still running 1 session. Run snow to return."},
		{name: "many", res: client.Result{Running: 2}, want: "Snow is still running 2 sessions. Run snow to return."},
		{name: "error", res: client.Result{Running: 2, Err: errors.New("failed")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := leaveLine(tt.res); got != tt.want {
				t.Fatalf("leaveLine(%+v) = %q, want %q", tt.res, got, tt.want)
			}
		})
	}
}
