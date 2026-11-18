package main

import (
	"fmt"

	"github.com/emmyfong/snow/internal/client"
)

func leaveLine(res client.Result) string {
	if res.Running == 0 || res.Err != nil {
		return ""
	}
	sessions := "sessions"
	if res.Running == 1 {
		sessions = "session"
	}
	return fmt.Sprintf("Snow is still running %d %s. Run snow to return.", res.Running, sessions)
}
