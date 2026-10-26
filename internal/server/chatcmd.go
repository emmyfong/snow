package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/pkg/api"
)

// switchModel makes the next turn of conv, in pane, use model and effort.
func (s *Server) switchModel(c *client, pane int, conv *agent.Conversation, model, effort string) error {
	if err := conv.SwitchModel(model, effort); err != nil {
		return err
	}
	// The agent's event confirms this a moment later. Setting it now closes
	// the gap: a message sent right after the switch must not see the old
	// model, or none.
	s.call(func(st *state) {
		if p := st.paneOf(c, pane); p != nil && p.chat != nil {
			p.chat.snap.Model, p.chat.snap.Effort = model, effort
		}
	})
	return nil
}

// setEffort makes the next turn use effort with the pane's model; an empty
// effort lets the provider choose. A level the model does not take is
// refused with the levels it does take.
func (s *Server) setEffort(c *client, pane int, conv *agent.Conversation, model, effort string) error {
	if model == "" {
		return errors.New("no model: choose one before setting the effort")
	}
	if effort != "" {
		levels, err := s.effortsOf(model)
		if err != nil {
			return fmt.Errorf("cannot check which effort levels %s takes: %w", model, err)
		}
		switch {
		case len(levels) == 0:
			return fmt.Errorf("%s takes no effort levels", model)
		case !slices.Contains(levels, effort):
			return fmt.Errorf("%s does not take effort %q. It takes: %s", model, effort, strings.Join(levels, ", "))
		}
	}
	return s.switchModel(c, pane, conv, model, effort)
}

// effortsOf returns the effort levels model ("provider/model") takes, from
// the model cache. A provider the cache has no list for is asked once. It
// runs on a client's reader goroutine, never on the model goroutine.
func (s *Server) effortsOf(model string) ([]string, error) {
	name, id, ok := api.ParseModelName(model)
	o := s.opts.Chat
	if !ok || o == nil {
		return nil, fmt.Errorf("%q is not provider/model", model)
	}
	find := func() (models []provider.Model, known bool) {
		s.call(func(st *state) { models, known = st.models.lists[name] })
		return models, known
	}
	models, known := find()
	if !known {
		p, ok := o.Providers.Get(name)
		if !ok {
			return nil, fmt.Errorf("no provider %s", name)
		}
		ctx, cancel := context.WithTimeout(context.Background(), modelFetchTimeout)
		fetched, err := p.Models(ctx)
		cancel()
		s.send(func(st *state) {
			st.models.done(name, fetched, err, o.now())
			st.broadcastModels()
		})
		if err != nil {
			return nil, err
		}
		models = fetched
	}
	for _, m := range models {
		if m.ID == id {
			levels := make([]string, len(m.Efforts))
			for i, e := range m.Efforts {
				levels[i] = string(e)
			}
			return levels, nil
		}
	}
	return nil, fmt.Errorf("%s does not offer %s", name, id)
}
