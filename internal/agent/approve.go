package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/tool"
)

// Mode says which tool calls wait for the user.
type Mode string

// The approval modes.
const (
	// ModeAsk asks before every tool that changes things. It is the default.
	ModeAsk Mode = "ask"
	// ModeAuto asks nothing inside the conversation folder.
	ModeAuto Mode = "auto"
)

// valid reports whether m is a mode this version knows.
func (m Mode) valid() bool { return m == ModeAsk || m == ModeAuto }

// Decision is the user's answer to an approval request.
type Decision string

// The answers to an approval request.
const (
	DecisionYes    Decision = Decision(convstore.DecisionYes)
	DecisionAlways Decision = Decision(convstore.DecisionAlways) // yes to this tool for the rest of this conversation
	DecisionNo     Decision = Decision(convstore.DecisionNo)
)

// Answer answers the approval request of the tool call ID.
type Answer struct {
	ID       string
	Decision Decision
	Reason   string // why not; goes back to the model with a no
}

// ErrNoApproval is the cause of an error for an answer that matches no
// waiting approval request.
var ErrNoApproval = errors.New("no approval request waits with that ID")

// verdict is what the approval rule says about a call.
type verdict int

const (
	allow verdict = iota
	ask
)

// approvalCase is everything the approval rule looks at.
type approvalCase struct {
	Tool    tool.Def
	Outside []string // paths of the call outside the conversation folder
	Mode    Mode
	Granted bool // the user said yes to this tool for the conversation
}

// decide is the approval rule. It is pure, so rules can grow here without
// touching the turn. A path outside the folder always asks: no mode and no
// grant covers it.
func decide(c approvalCase) verdict {
	switch {
	case len(c.Outside) > 0:
		return ask
	case !c.Tool.Changes, c.Mode == ModeAuto, c.Granted:
		return allow
	}
	return ask
}

// approver returns the approval step for one tool call. It runs on the
// turn's goroutine, which alone reads and writes the grants.
func (c *Conversation) approver(id string) tool.Approver {
	return func(ctx context.Context, t tool.Tool, input json.RawMessage) (bool, string) {
		def := t.Def()
		env := c.env()
		outside := env.Folder.Outside(t, input)
		if decide(approvalCase{Tool: def, Outside: outside, Mode: c.Mode(), Granted: c.grants[def.Name]}) == allow {
			return true, ""
		}
		req := &ApprovalRequest{ID: id, Tool: def.Name, Input: input, Outside: outside}
		if p, ok := t.(tool.Previewer); ok {
			if ch, err := p.Preview(env, input); err == nil {
				req.Diff = &Diff{Path: ch.Path, Old: ch.Old, New: ch.New}
			}
		}
		a, ok := c.wait(ctx, req)
		if !ok {
			return false, "interrupted by the user"
		}
		rec := &convstore.Approval{At: c.now(), ToolUseID: id, Decision: convstore.Decision(a.Decision), Reason: a.Reason}
		if err := c.w.Append(rec); err != nil {
			return false, fmt.Sprintf("the answer could not be saved: %v", err)
		}
		switch a.Decision {
		case DecisionAlways:
			c.grants[def.Name] = true
			return true, ""
		case DecisionYes:
			return true, ""
		}
		if a.Reason == "" {
			return false, "the user said no"
		}
		return false, a.Reason
	}
}

// wait shows req to the user and waits, with no timeout, for the answer.
// It reports false when the turn ends first.
func (c *Conversation) wait(ctx context.Context, req *ApprovalRequest) (Answer, bool) {
	reply := make(chan Answer, 1)
	select {
	case c.cmds <- waitCmd{req: req, reply: reply}:
	case <-ctx.Done():
		return Answer{}, false
	}
	c.emit(req)
	c.emit(&StateChanged{State: StateApproval})
	select {
	case a := <-reply:
		// An answer and an interrupt can be ready together; the interrupt
		// wins, so nothing runs after the user pressed Esc.
		if ctx.Err() != nil {
			return Answer{}, false
		}
		c.emit(&StateChanged{State: StateTool})
		return a, true
	case <-ctx.Done():
		return Answer{}, false
	}
}

// env is what the conversation's tools get.
func (c *Conversation) env() tool.Env {
	return tool.Env{Folder: c.cfg.Folder, Shell: c.cfg.Shell, HideEnv: c.cfg.HideEnv}
}
