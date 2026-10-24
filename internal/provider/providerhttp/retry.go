package providerhttp

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// MaxRetryAfter caps a vendor's retry time, so a bad header cannot ask the
// user to wait for days.
const MaxRetryAfter = time.Hour

// RetryAfter reads a retry-after header in seconds. Zero means unknown.
func RetryAfter(v string) time.Duration {
	n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0
	}
	return Seconds(n)
}

// Seconds turns a vendor's retry time in seconds into a duration, capped
// at MaxRetryAfter. Zero means unknown, as does a negative or invalid n.
func Seconds(n float64) time.Duration {
	if math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
		return 0
	}
	if n >= MaxRetryAfter.Seconds() {
		return MaxRetryAfter
	}
	return time.Duration(n * float64(time.Second))
}
