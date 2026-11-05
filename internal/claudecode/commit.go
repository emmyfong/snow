package claudecode

import (
	"context"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/git"
	"github.com/emmyfong/snow/pkg/api"
)

// commitCall matches a shell command that runs git commit, also after
// options such as -C and in a list such as "cd x && git commit". It also
// matches some reads, such as git log --grep commit; those leave HEAD
// where it was, so they mark nothing.
var commitCall = regexp.MustCompile(`\bgit\b[^\n;&|]*\bcommit\b`)

// Hook budgets for a tool call that runs git commit. Reading HEAD may
// start git through wsl.exe, which is slow to start, so such a call gets
// more than sendBudget. Claude Code waits for the hook before the tool
// runs; it costs this only on git commit calls. sendMargin is kept for
// the send after the read.
const (
	commitBudget = 2 * time.Second
	sendMargin   = 250 * time.Millisecond
)

// isCommitCall reports whether e is a hook around a Bash tool call that
// runs git commit.
func isCommitCall(e event) bool {
	return (e.Name == EventPreToolUse || e.Name == EventPostToolUse) &&
		e.Tool == "Bash" && e.Cwd != "" && commitCall.MatchString(e.Command)
}

// readCommit reads HEAD around a Bash tool call that runs git commit: HEAD
// before it, or HEAD and its first parent after it, giving up at until.
// distro is the WSL distribution the agent runs in, or empty. It returns
// nil for any other event. A HEAD git cannot read is left empty, which
// marks nothing.
func readCommit(e event, distro string, until time.Time) *api.AgentCommit {
	if !isCommitCall(e) {
		return nil
	}
	ctx, cancel := context.WithDeadline(context.Background(), until)
	defer cancel()
	head, parent := readHead(ctx, headRepo(runtime.GOOS, e.Cwd, distro))
	if e.Name == EventPreToolUse {
		return &api.AgentCommit{Phase: api.CommitBefore, Before: head}
	}
	return &api.AgentCommit{Phase: api.CommitAfter, After: head, Parent: parent}
}

// headRepo is the repository at cwd as snow hook on goos reaches it. A
// Claude Code in WSL starts the Windows snow.exe, and its cwd is a Linux
// path, which only git inside the distribution can read.
func headRepo(goos, cwd, distro string) git.Repo {
	if goos == "windows" && strings.HasPrefix(cwd, "/") && distro != "" {
		return git.Repo{Path: cwd, Distro: distro}
	}
	return git.Repo{Path: cwd}
}

// readHead returns HEAD in r and HEAD's first parent, or empty strings
// when git cannot read them.
func readHead(ctx context.Context, r git.Repo) (head, parent string) {
	out, err := r.Run(ctx, r.Path, "rev-list", "--parents", "-n", "1", "HEAD")
	if err != nil {
		return "", ""
	}
	f := strings.Fields(out)
	if len(f) > 0 && api.IsCommitHash(f[0]) {
		head = f[0]
	}
	if len(f) > 1 && api.IsCommitHash(f[1]) {
		parent = f[1]
	}
	return head, parent
}
