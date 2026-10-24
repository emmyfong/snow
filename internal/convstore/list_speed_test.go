package convstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeHistory writes n conversation files of the given number of turns,
// about 5 KB of tool output each, and returns their store. 20 turns make
// about 100 KB, a long session.
func writeHistory(t testing.TB, n, turns int) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "conversations"))
	if err != nil {
		t.Fatal(err)
	}
	output := strings.Repeat("some tool output line\n", 100)
	var b strings.Builder
	for i := range n {
		b.Reset()
		id := fmt.Sprintf("20261023-191500-%08x", i)
		start := at.Add(time.Duration(i) * time.Minute)
		recs := []Record{&Meta{Version: Version, ID: id, Folder: "/home/u/p", Created: start, Model: "anthropic/claude-opus-5-5"}}
		for turn := range turns {
			recs = append(recs,
				&User{At: start, Text: fmt.Sprint("question ", turn)},
				&Assistant{At: start, Blocks: []Block{{Kind: BlockText, Text: "answer"}, {Kind: BlockToolUse, ID: "c", Name: "read"}}, Stop: StopToolUse},
				&ToolResult{At: start, ToolUseID: "c", Content: output},
				&Assistant{At: start, Blocks: []Block{{Kind: BlockText, Text: "done"}}, Stop: "end_turn"})
		}
		for _, r := range recs {
			line, err := Encode(r)
			if err != nil {
				t.Fatal(err)
			}
			b.Write(line)
		}
		if err := os.WriteFile(filepath.Join(s.dir, id+ext), []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// listLimit is how long ListAll may take for 1000 files in TestListSpeed.
// Measured on 2026-10-24, 16 cores: 1000 files of 100 KB list in 0.11 s on
// Linux and 0.25 s on Windows (NTFS); 1000 files of five turns in 0.1 s.
// The limit leaves room for slow CI runners.
const listLimit = 2 * time.Second

// TestListSpeed lists 1000 conversations of five turns each under
// listLimit. BenchmarkListAll measures the 100 KB case.
func TestListSpeed(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 1000 files")
	}
	s := writeHistory(t, 1000, 5)
	// Windows antivirus scans each new file on its first open, which takes
	// over a second for 1000 files. That is a cost of files just written,
	// not of the list, so one list runs before the timed one.
	if _, err := s.ListAll(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	es, err := s.ListAll()
	took := time.Since(start)
	if err != nil || len(es) != 1000 {
		t.Fatalf("%d entries, %v", len(es), err)
	}
	if raceDetector {
		t.Logf("listed 1000 files in %v with the race detector; no limit", took)
		return
	}
	if took > listLimit {
		t.Fatalf("listing 1000 files took %v, limit %v", took, listLimit)
	}
	t.Logf("listed 1000 files in %v", took)
}

func BenchmarkListAll(b *testing.B) {
	s := writeHistory(b, 1000, 20)
	b.ResetTimer()
	for range b.N {
		if _, err := s.ListAll(); err != nil {
			b.Fatal(err)
		}
	}
}
