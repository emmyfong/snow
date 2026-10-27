package server

import "time"

// clockSlack is how far ahead of the server's clock a report may be
// stamped and still keep its own time.
const clockSlack = 5 * time.Second

// notAfterNow returns at, or the server's time now when at is more than
// clockSlack ahead of it. A report from a clock that runs ahead would
// otherwise make every later report look older and be ignored.
func notAfterNow(at time.Time) time.Time {
	if now := time.Now(); at.After(now.Add(clockSlack)) {
		return now
	}
	return at
}
