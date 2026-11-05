package gitpane

import (
	"strconv"
	"strings"

	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// owners returns the agents that work in wt, of the repository's
// worktrees wts. An agent belongs to the worktree Snow made for it, else
// to the deepest worktree that holds its folder: the main checkout holds
// the linked worktrees' folders too.
func owners(wt api.GitWorktree, wts []api.GitWorktree, agents []api.AgentInfo) []api.AgentInfo {
	var out []api.AgentInfo
	for _, a := range agents {
		folder := a.Worktree
		if folder == "" {
			folder = a.Folder
		}
		best, bestLen := -1, -1
		for i, w := range wts {
			if p := normPath(w.Path); within(normPath(folder), p) && len(p) > bestLen {
				best, bestLen = i, len(p)
			}
		}
		if best >= 0 && samePath(wts[best].Path, wt.Path) {
			out = append(out, a)
		}
	}
	return out
}

// normPath puts a path in one form for comparing: forward slashes, no
// slash at the end, and lower case for a Windows path, whose file system
// ignores case. Agents report a Windows folder with backslashes; git lists
// it with forward slashes.
func normPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	if windowsPath(p) {
		p = strings.ToLower(p)
	}
	return p
}

// windowsPath reports whether p, with forward slashes, is a Windows path:
// one that starts with a drive letter or is a UNC path.
func windowsPath(p string) bool {
	if strings.HasPrefix(p, "//") {
		return true
	}
	return len(p) >= 2 && p[1] == ':' && (p[0]|0x20 >= 'a' && p[0]|0x20 <= 'z')
}

// within reports whether folder is dir or below it. Both are in normPath
// form.
func within(folder, dir string) bool {
	return folder == dir || strings.HasPrefix(folder, strings.TrimSuffix(dir, "/")+"/")
}

func samePath(a, b string) bool { return normPath(a) == normPath(b) }

// ownerText names who works in wt: the first owner's window by number,
// with its name when that is not just the branch (an agent's window is
// named after its branch), and how many more owners there are.
func ownerText(wt api.GitWorktree, own []api.AgentInfo) string {
	if len(own) == 0 {
		return ""
	}
	name := "window " + strconv.Itoa(own[0].Window)
	if n := own[0].WindowName; n != "" && n != wt.Branch {
		name += " " + ui.PlainLine(n)
	}
	if len(own) > 1 {
		name += " +" + strconv.Itoa(len(own)-1)
	}
	return name
}

// openWorktree goes to the agent that works in wt, or else opens a
// terminal in a new window in its folder.
func (m *Model) openWorktree(wt api.GitWorktree) api.Message {
	if own := owners(wt, m.st.Worktrees, m.agents); len(own) > 0 {
		return &api.Command{Op: api.OpFocus, Target: own[0].Pane}
	}
	return &api.Command{Op: api.OpNewWindow, Target: m.pane, Args: map[string]string{api.ArgKind: api.PaneTerminal, api.ArgFolder: wt.Path}}
}
