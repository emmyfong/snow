package api

import "time"

// setHandshakeTimeout changes the handshake timeout for a test and returns a
// function that restores it.
func setHandshakeTimeout(d time.Duration) func() {
	old := handshakeTimeout
	handshakeTimeout = d
	return func() { handshakeTimeout = old }
}
