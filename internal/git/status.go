package git

import (
	"context"
	"strconv"
	"strings"
)

// MaxChanges bounds the changes Status returns. A folder with more, such
// as one with a build output git does not ignore, says how many it left out.
const MaxChanges = 1000

// Change is one changed file. Index and Work are git's one-letter codes for
// the staged and the unstaged change: M, A, D, R, C, T, or U for a merge
// conflict, and empty for none. An untracked file has Work "?".
type Change struct {
	Path     string // from the top of the repository, with forward slashes
	Orig     string // a rename's or copy's old path
	Index    string
	Work     string
	Conflict bool
}

// Staged reports whether c has a staged change.
func (c Change) Staged() bool { return c.Index != "" && !c.Conflict }

// Untracked reports whether c is a file git does not track.
func (c Change) Untracked() bool { return c.Work == "?" }

// Status is the state of a repository's checkout.
type Status struct {
	Branch   string // empty on a detached HEAD
	Detached bool
	Initial  bool // the branch has no commit yet
	Upstream string
	Ahead    int
	Behind   int
	Changes  []Change
	Omitted  int // changes past MaxChanges
}

// Status returns the state of the checkout that holds r.Path.
func (r Repo) Status(ctx context.Context) (Status, error) {
	out, err := r.Run(ctx, r.Path, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=all")
	if err != nil {
		return Status{}, r.Classify(err)
	}
	return parseStatus(out), nil
}

// parseStatus reads git status --porcelain=v2 --branch -z: records split by
// NUL, where a rename's record is followed by one more, its old path.
func parseStatus(out string) Status {
	var st Status
	recs := strings.Split(out, "\x00")
	for i := 0; i < len(recs); i++ {
		rec := recs[i]
		if rec == "" {
			continue
		}
		var c Change
		switch rec[0] {
		case '#':
			st.header(rec)
			continue
		case '1':
			f := strings.SplitN(rec, " ", 9)
			if len(f) < 9 {
				continue
			}
			c = Change{Path: f[8], Index: code(f[1][0]), Work: code(f[1][1])}
		case '2':
			f := strings.SplitN(rec, " ", 10)
			if len(f) < 10 {
				continue
			}
			c = Change{Path: f[9], Index: code(f[1][0]), Work: code(f[1][1])}
			if i+1 < len(recs) {
				i++
				c.Orig = recs[i]
			}
		case 'u':
			f := strings.SplitN(rec, " ", 11)
			if len(f) < 11 {
				continue
			}
			c = Change{Path: f[10], Index: "U", Work: "U", Conflict: true}
		case '?':
			c = Change{Path: strings.TrimPrefix(rec, "? "), Work: "?"}
		default:
			continue // "!" is an ignored file, which the command does not ask for
		}
		if len(st.Changes) == MaxChanges {
			st.Omitted++
			continue
		}
		st.Changes = append(st.Changes, c)
	}
	return st
}

// header reads one "# branch.*" record.
func (st *Status) header(rec string) {
	key, val, _ := strings.Cut(strings.TrimPrefix(rec, "# "), " ")
	switch key {
	case "branch.oid":
		st.Initial = val == "(initial)"
	case "branch.head":
		if val == "(detached)" {
			st.Detached = true
		} else {
			st.Branch = val
		}
	case "branch.upstream":
		st.Upstream = val
	case "branch.ab":
		a, b, _ := strings.Cut(val, " ")
		st.Ahead, _ = strconv.Atoi(strings.TrimPrefix(a, "+"))
		st.Behind, _ = strconv.Atoi(strings.TrimPrefix(b, "-"))
	}
}

// code turns porcelain's "." for no change into "".
func code(b byte) string {
	if b == '.' {
		return ""
	}
	return string(b)
}
