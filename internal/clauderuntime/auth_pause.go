package clauderuntime

import "context"

// PauseForAuth atomically refuses active work or prevents new submissions and
// stops an idle cached runtime. Credential changes can then occur without a
// process continuing to use an earlier account. Call ResumeAfterAuth afterward.
func (c *Conversation) PauseForAuth(ctx context.Context) error {
	return c.authCommand(ctx, conversationCommand{authPause: true, reply: make(chan error, 1)})
}

// ResumeAfterAuth permits explicit input again. It never launches a runtime,
// releases queued drafts or repeats a user prompt.
func (c *Conversation) ResumeAfterAuth(ctx context.Context) error {
	return c.authCommand(ctx, conversationCommand{authResume: true, reply: make(chan error, 1)})
}

func (c *Conversation) authCommand(ctx context.Context, command conversationCommand) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case c.commands <- command:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return ErrClosed
	}
	select {
	case err := <-command.reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return ErrClosed
	}
}

func (s *conversationState) finishAuthPause(err error) {
	for _, reply := range s.authReplies {
		reply <- err
	}
	s.authReplies = nil
}
