package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/tool"
)

// QuestionReply answers the open question ID: the labels picked, or the
// text typed instead.
type QuestionReply struct {
	ID     string
	Picked []string
	Other  string
}

// ErrNoQuestion is the cause of an error for a reply that matches no open
// question.
var ErrNoQuestion = errors.New("no question is open with that ID")

// errInterrupted is what a tool that waited for the user returns when the
// turn ended first.
var errInterrupted = errors.New("interrupted by the user")

// asking is the open question. reply is nil when no turn waits for it:
// after an interrupt, or after a resume. An answer then starts a turn.
type asking struct {
	q     *Question
	reply chan<- tool.Reply
}

// callUser is the user as the tools of one call see them.
type callUser struct {
	c  *Conversation
	id string // the tool call's ID
}

// Ask saves q, shows it, and waits for the answer, which it saves too.
// It runs on the turn's goroutine.
func (u callUser) Ask(ctx context.Context, q tool.Question) (tool.Reply, error) {
	c := u.c
	rec := &convstore.Question{At: c.now(), ToolUseID: u.id, Text: q.Text, Multi: q.Multi}
	for _, o := range q.Options {
		rec.Options = append(rec.Options, convstore.QuestionOption{Label: o.Label, Description: o.Description, Preview: o.Preview})
	}
	ev := QuestionOf(rec)
	if err := c.w.Append(rec); err != nil {
		return tool.Reply{}, fmt.Errorf("the question could not be saved: %w", err)
	}
	// The saved question is open from here on, so the loop and the user
	// must hear of it even when the turn was interrupted already. The loop
	// does not return while a turn runs, so this send is always received.
	reply := make(chan tool.Reply, 1)
	c.cmds <- askCmd{q: ev, reply: reply}
	c.emit(ev)
	c.emit(&StateChanged{State: StateApproval})
	select {
	case r := <-reply:
		return u.answered(r)
	case <-ctx.Done():
		// The loop hands over a reply before it handles the interrupt, so
		// one that came first is here now. Unlike an approval's, an answer
		// the loop took is kept: dropping it would leave a question the
		// file holds open and the loop does not.
		select {
		case r := <-reply:
			return u.answered(r)
		default:
			return tool.Reply{}, errInterrupted
		}
	}
}

// answered saves the answer r and reports it.
func (u callUser) answered(r tool.Reply) (tool.Reply, error) {
	c := u.c
	if err := c.w.Append(&convstore.Answer{At: c.now(), ToolUseID: u.id, Picked: r.Picked, Other: r.Other}); err != nil {
		return tool.Reply{}, fmt.Errorf("the answer could not be saved: %w", err)
	}
	c.emit(&QuestionAnswered{ID: u.id})
	c.emit(&StateChanged{State: StateTool})
	return r, nil
}

// Reply answers the open question. When a turn waits for it, the turn
// goes on with the answer. Otherwise, after an interrupt or a resume, the
// answer starts a new turn that gives the model the question and the
// answer. A reply that matches no open question gives an error wrapping
// ErrNoQuestion; one that does not fit the question, ErrInvalid; one that
// comes while another turn runs, ErrBusy.
func (c *Conversation) Reply(r QuestionReply) error {
	res := make(chan error, 1)
	if err := c.post(replyCmd{r, res}); err != nil {
		return err
	}
	return <-res
}

// Question returns the open question, or nil. It waits for the
// conversation's goroutine, so the server's own goroutine must not call it.
func (c *Conversation) Question() *Question {
	res := make(chan *Question, 1)
	if c.post(questionCmd{res}) != nil {
		return nil
	}
	return <-res
}

// reply handles a replyCmd on the loop's goroutine.
func (c *Conversation) reply(l *loopState, r QuestionReply) error {
	a := l.asking
	if a == nil || a.q.ID != r.ID {
		return fmt.Errorf("reply %q: %w", r.ID, ErrNoQuestion)
	}
	picked, err := fitReply(a.q, r)
	if err != nil {
		return fmt.Errorf("reply %q: %w: %w", r.ID, ErrInvalid, err)
	}
	if a.reply != nil {
		a.reply <- tool.Reply{Picked: picked, Other: r.Other}
		l.asking = nil
		return nil
	}
	if l.cancel != nil || l.closing || l.holding() {
		return fmt.Errorf("reply %q: %w", r.ID, ErrBusy)
	}
	l.asking = nil
	ans := &convstore.Answer{At: c.now(), ToolUseID: r.ID, Picked: picked, Other: r.Other}
	c.beginWith(l, answerMessage(a.q, picked, r.Other), ans)
	return nil
}

// fitReply checks r against q and returns the picked labels in the order
// of q's options.
func fitReply(q *Question, r QuestionReply) ([]string, error) {
	if r.Other != "" {
		switch {
		case len(r.Picked) > 0:
			return nil, errors.New("a reply picks options or types an answer, not both")
		case strings.TrimSpace(r.Other) == "":
			return nil, errors.New("the typed answer is blank")
		}
		return nil, nil
	}
	var picked []string
	for _, o := range q.Options {
		if slices.Contains(r.Picked, o.Label) {
			picked = append(picked, o.Label)
		}
	}
	switch {
	case len(picked) != len(r.Picked):
		return nil, fmt.Errorf("%q are not all options of the question, or one is picked twice", r.Picked)
	case len(picked) == 0:
		return nil, errors.New("nothing is picked")
	case len(picked) > 1 && !q.Multi:
		return nil, errors.New("the question takes one answer")
	}
	return picked, nil
}

// answerMessage is the user's message that answers q after its turn
// ended.
func answerMessage(q *Question, picked []string, other string) string {
	answer := other
	if answer == "" {
		answer = strings.Join(picked, ", ")
	}
	return "You asked: " + q.Text + "\nMy answer: " + answer
}

// QuestionOf turns a saved question into its event, or nil for nil.
func QuestionOf(q *convstore.Question) *Question {
	if q == nil {
		return nil
	}
	out := &Question{ID: q.ToolUseID, Text: q.Text, Multi: q.Multi}
	for _, o := range q.Options {
		out.Options = append(out.Options, Option{Label: o.Label, Description: o.Description, Preview: o.Preview})
	}
	return out
}
