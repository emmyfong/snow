package claudecode

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// commitCall matches a shell command that runs git commit, also after
// options such as -C and in a list such as "cd x && git commit".
var commitCall = regexp.MustCompile(`\bgit\b[^\n;&|]*\bcommit\b`)

// headWait bounds the git that snow hook runs. Claude Code waits for the
// hook before the tool runs, so the read before a commit is done before
// the commit starts.
const headWait = 250 * time.Millisecond

// readCommit reads HEAD around a Bash tool call that runs git commit: HEAD
// before it, or HEAD and its first parent after it. It returns nil for
// any other event, and for a folder git cannot read; an agent's commit
// then simply has no mark.
func readCommit(e event, deadline time.Time) *api.AgentCommit {
	if e.Tool != "Bash" || e.Cwd == "" || !commitCall.MatchString(e.Command) {
		return nil
	}
	if e.Name != EventPreToolUse && e.Name != EventPostToolUse {
		return nil
	}
	if soon := time.Now().Add(headWait); soon.Before(deadline) {
		deadline = soon
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	head, parent := readHead(ctx, e.Cwd)
	c := &api.AgentCommit{Folder: e.Cwd}
	if e.Name == EventPreToolUse {
		c.Before = head
	} else {
		c.After, c.Parent = head, parent
	}
	if c.Validate() != nil {
		return nil
	}
	return c
}

// readHead returns HEAD in folder and HEAD's first parent, or empty
// strings when git cannot read them.
func readHead(ctx context.Context, folder string) (head, parent string) {
	cmd := exec.CommandContext(ctx, "git", "-C", folder, "rev-list", "--parents", "-n", "1", "HEAD")
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	f := strings.Fields(string(out))
	if len(f) > 0 && api.IsCommitHash(f[0]) {
		head = f[0]
	}
	if len(f) > 1 && api.IsCommitHash(f[1]) {
		parent = f[1]
	}
	return head, parent
}
