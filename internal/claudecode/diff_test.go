package claudecode

import (
	"strings"
	"testing"
)

func TestUnifiedDiff(t *testing.T) {
	a := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	tests := []struct {
		name, b, want string
	}{
		{"no change", a, ""},
		{"one line changed", strings.Replace(a, "5\n", "five\n", 1), "--- old\n+++ new\n@@ -2,7 +2,7 @@\n 2\n 3\n 4\n-5\n+five\n 6\n 7\n 8\n"},
		{"added at the end", a + "11\n", "--- old\n+++ new\n@@ -8,3 +8,4 @@\n 8\n 9\n 10\n+11\n"},
		{"from nothing", "", "--- old\n+++ new\n@@ -1,10 +0,0 @@\n-1\n-2\n-3\n-4\n-5\n-6\n-7\n-8\n-9\n-10\n"},
	}
	for _, tt := range tests {
		if got := unifiedDiff([]byte(a), []byte(tt.b), "old", "new"); got != tt.want {
			t.Errorf("%s: diff =\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
	far := strings.Replace(strings.Replace(a, "1\n", "one\n", 1), "10\n", "ten\n", 1)
	if got := unifiedDiff([]byte(a), []byte(far), "old", "new"); strings.Count(got, "@@ ") != 2 {
		t.Errorf("changes far apart give one hunk each:\n%s", got)
	}
	if got := unifiedDiff([]byte("a\r\nb\r\n"), []byte("a\r\nc\r\n"), "old", "new"); strings.Contains(got, "\r") {
		t.Errorf("diff keeps carriage returns: %q", got)
	}
}
