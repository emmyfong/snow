package git

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/gittest"
)

// TestParseLog: records with a subject that holds separators of other
// kinds, and a broken record that is skipped.
func TestParseLog(t *testing.T) {
	out := "aaaa\x1faa\x1fAda\x1f1700000000\x1ffix: a | b; c\x1e\nbbbb\x1fbb\x1fBo\x1fx\x1fsecond\x1e\nbroken\x1e\n"
	cs := parseLog(out)
	if len(cs) != 2 {
		t.Fatalf("%d commits: %+v", len(cs), cs)
	}
	if cs[0] != (Commit{Hash: "aaaa", Short: "aa", Author: "Ada", When: time.Unix(1700000000, 0), Subject: "fix: a | b; c"}) {
		t.Errorf("first %+v", cs[0])
	}
	if cs[1].Subject != "second" || !cs[1].When.IsZero() {
		t.Errorf("second %+v", cs[1])
	}
}

// TestLogReal: the newest commits come first, as many as asked for.
func TestLogReal(t *testing.T) {
	dir := repo(t)
	write(t, dir, "b.txt", "b\n")
	gittest.Run(t, dir, "add", "b.txt")
	gittest.Run(t, dir, "commit", "-q", "-m", "second")
	// A signature check that git prints before each record must not reach
	// the parser.
	gittest.Run(t, dir, "config", "log.showSignature", "true")
	cs, err := Repo{Path: dir}.Log(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Subject != "second" || cs[0].Author != "Test" || cs[0].Hash != gittest.Run(t, dir, "rev-parse", "HEAD") {
		t.Errorf("log %+v", cs)
	}
}

// TestHistoryPreservesGitMetadata: reading the history leaves the config,
// the refs, and the commit's bytes as they were.
func TestHistoryPreservesGitMetadata(t *testing.T) {
	dir := repo(t)
	snapshot := func() [][]byte {
		cfg, err := os.ReadFile(filepath.Join(dir, ".git", "config"))
		if err != nil {
			t.Fatal(err)
		}
		return [][]byte{cfg, []byte(gittest.Run(t, dir, "show-ref", "--head")), []byte(gittest.Run(t, dir, "cat-file", "-p", "HEAD"))}
	}
	before := snapshot()
	if _, err := (Repo{Path: dir}).Log(context.Background(), 20); err != nil {
		t.Fatal(err)
	}
	for i, now := range snapshot() {
		if !bytes.Equal(now, before[i]) {
			t.Errorf("part %d changed:\n%s\nwas:\n%s", i, now, before[i])
		}
	}
}
