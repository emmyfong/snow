package git

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// Commit is one commit of the history, as git log has it.
type Commit struct {
	Hash    string // the full object name
	Short   string // the abbreviated one git prints
	Author  string
	When    time.Time // the author date
	Subject string    // the first line of the message
}

// Log returns the newest n commits of HEAD, newest first. It only reads:
// it changes no config, ref, or commit.
func (r Repo) Log(ctx context.Context, n int) ([]Commit, error) {
	out, err := r.Run(ctx, r.Path, "log", "-n", strconv.Itoa(n), "--no-color",
		"--format=%H%x1f%h%x1f%an%x1f%at%x1f%s%x1e", "HEAD", "--")
	if err != nil {
		return nil, r.Classify(err)
	}
	return parseLog(out), nil
}

// parseLog reads the records Log asks for: fields split by the unit
// separator, records by the record separator.
func parseLog(out string) []Commit {
	var cs []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		f := strings.Split(strings.TrimLeft(rec, "\r\n"), "\x1f")
		if len(f) != 5 || f[0] == "" {
			continue
		}
		c := Commit{Hash: f[0], Short: f[1], Author: f[2], Subject: cut(f[4], MaxLineBytes)}
		if sec, err := strconv.ParseInt(f[3], 10, 64); err == nil {
			c.When = time.Unix(sec, 0)
		}
		cs = append(cs, c)
	}
	return cs
}
