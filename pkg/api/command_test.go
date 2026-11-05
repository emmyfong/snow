package api

import (
	"errors"
	"testing"
)

func TestCommandValidate(t *testing.T) {
	tests := []struct {
		cmd Command
		ok  bool
	}{
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight}}, true},
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirDown, ArgProfile: "Ubuntu", ArgFolder: "/src"}}, true},
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: "sideways"}}, false},
		{Command{Op: OpSplit, Args: map[string]string{ArgDir: DirRight}}, false},
		{Command{Op: OpResize, Target: 1, Args: map[string]string{ArgDir: DirLeft, ArgCells: "5"}}, true},
		{Command{Op: OpResize, Target: 1, Args: map[string]string{ArgDir: DirLeft, ArgCells: "five"}}, false},
		{Command{Op: OpResize, Target: 1, Args: map[string]string{ArgDir: DirLeft, ArgCells: "0"}}, false},
		{Command{Op: OpFocus, Target: 1, Args: map[string]string{ArgDir: DirUp}}, true},
		{Command{Op: OpFocus, Target: 3}, true},
		{Command{Op: OpClose, Target: 2}, true},
		{Command{Op: OpZoom, Target: 2}, true},
		{Command{Op: OpEven, Target: 2}, true},
		{Command{Op: OpSwap, Target: 2, Args: map[string]string{ArgDir: DirRight}}, true},
		{Command{Op: OpSwap, Target: 2}, false},
		{Command{Op: OpNewWindow}, true},
		{Command{Op: OpRenameWindow, Target: 1, Args: map[string]string{ArgName: "logs"}}, true},
		{Command{Op: OpRenameWindow, Target: 1}, false},
		{Command{Op: OpSelectWindow, Target: 2}, true},
		{Command{Op: OpSelectWindow}, true}, // window 0, the first
		{Command{Op: OpSelectWindow, Target: -1}, false},
		{Command{Op: OpRenameWindow, Args: map[string]string{ArgName: "dev"}}, true},
		{Command{Op: OpNextWindow}, true},
		{Command{Op: OpPrevWindow}, true},
		{Command{Op: OpCloseWindow, Target: 1}, true},
		{Command{Op: "explode", Target: 1}, false},
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight, ArgAgent: AgentClaude, ArgFolder: "/src"}}, true},
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight, ArgAgent: "gemini"}}, false},
		{Command{Op: OpNewWindow, Args: map[string]string{ArgAgent: AgentClaude}}, false},
		// An agent picks its own profile, and its pane is a terminal.
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight, ArgAgent: AgentClaude, ArgProfile: "Ubuntu"}}, false},
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight, ArgAgent: AgentClaude, ArgKind: PaneChat}}, false},
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight, ArgAgent: AgentClaude, ArgKind: PaneTerminal}}, true},
		{Command{Op: OpAgentNew, Target: 1, Args: map[string]string{ArgProfile: "Claude Code", ArgBranch: "agent/2"}}, true},
		{Command{Op: OpAgentNew, Target: 1, Args: map[string]string{ArgProfile: "Claude Code"}}, true},
		{Command{Op: OpAgentNew, Target: 1}, false},
		{Command{Op: OpAgentNew, Args: map[string]string{ArgProfile: "Claude Code"}}, false},
		{Command{Op: OpClose, Target: 1, Args: map[string]string{ArgWorktree: WorktreeRemove}}, true},
		{Command{Op: OpClose, Target: 1, Args: map[string]string{ArgWorktree: "delete"}}, false},
		{Command{Op: OpCloseWindow, Target: 1, Args: map[string]string{ArgWorktree: WorktreeRemove}}, false},
		{Command{Op: OpAgentNew, Target: 1, Args: map[string]string{ArgProfile: "Claude Code", ArgBranch: "fix", ArgRole: "reviewer"}}, true},
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight, ArgBranch: "fix"}}, false},
		{Command{Op: OpNewWindow, Args: map[string]string{ArgRole: "reviewer"}}, false},
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight, ArgAgent: AgentClaude, ArgKind: "files"}}, false},
		// A well-formed kind passes; the server refuses one it does not have.
		{Command{Op: OpSplit, Target: 1, Args: map[string]string{ArgDir: DirRight, ArgKind: "files"}}, true},
		{Command{Op: OpNewWindow, Args: map[string]string{ArgKind: "Files"}}, false},
		{Command{Op: OpNewWindow, Args: map[string]string{ArgKind: ""}}, false},
		{Command{Op: OpOpen, Target: 1, Args: map[string]string{ArgKind: "files"}}, true},
		{Command{Op: OpLayout, Target: 1, Args: map[string]string{ArgName: "ide"}}, true},
		{Command{Op: OpLayout, Target: 1}, false},
		{Command{Op: OpLayout, Args: map[string]string{ArgName: "ide"}}, false},
		{Command{Op: OpOpen, Args: map[string]string{ArgKind: "files"}}, false},
		{Command{Op: OpOpen, Target: 1}, false},
		{Command{Op: OpOpen, Target: 1, Args: map[string]string{ArgKind: PaneTerminal}}, true},
		{Command{Op: OpOpen, Target: 1, Args: map[string]string{ArgKind: "files", ArgAgent: AgentClaude}}, false},
	}
	for _, tt := range tests {
		err := tt.cmd.Validate()
		if (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrInvalidCommand)) {
			t.Errorf("%+v.Validate() = %v, want ok=%v", tt.cmd, err, tt.ok)
		}
	}
}

func TestCommandCells(t *testing.T) {
	c := Command{Op: OpResize, Target: 1, Args: map[string]string{ArgDir: DirLeft, ArgCells: "7"}}
	if n := c.Cells(); n != 7 {
		t.Fatalf("Cells() = %d, want 7", n)
	}
}
