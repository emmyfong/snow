package command

import (
	"slices"
	"strings"
)

// fuzzyScore reports whether the runes of q appear in s in order, ignoring
// case, and how well they match: a match at the start, runs of adjacent
// runes, and a whole prefix score higher; gaps score lower.
func fuzzyScore(q, s string) (int, bool) {
	q, s = strings.ToLower(q), strings.ToLower(s)
	if q == "" {
		return 0, true
	}
	rs := []rune(s)
	score, at, prev := 0, 0, -2
	for _, r := range q {
		i := slices.Index(rs[at:], r)
		if i < 0 {
			return 0, false
		}
		i += at
		switch i {
		case 0:
			score += 10
		case prev + 1:
			score += 5
		default:
			score -= i - at
		}
		prev, at = i, i+1
	}
	if strings.HasPrefix(s, q) {
		score += 20
	}
	return score, true
}
