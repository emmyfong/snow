package server

import (
	"errors"
	"fmt"
	"slices"

	"github.com/emmyfong/snow/pkg/api"
)

// ErrUnknownKind is returned for a pane kind the server does not have.
var ErrUnknownKind = errors.New("unknown pane kind")

// ErrDuplicateKind is returned when two pane kinds share a name.
var ErrDuplicateKind = errors.New("duplicate pane kind")

// sidePane is the server half of a pane kind other than terminal and chat,
// such as a file tree. Its methods run on the model goroutine and must not
// block: a kind that reads files or runs programs does that in its own
// goroutine and calls its sideContext's Changed when its state moves on.
type sidePane interface {
	// State returns the pane's whole state for its clients.
	State() api.SideState
	// Act does action with args, from a client. An error goes back to that
	// client; the state goes to every client after a success.
	Act(action string, args map[string]string) error
	// Close ends the pane. It may wait, so it never runs on the model
	// goroutine.
	Close() error
}

// sideContext is what a side pane gets when it opens.
type sideContext struct {
	Pane   int
	Folder string // the folder of the pane it opened from, or the session's
	// Distro is the WSL distribution Folder belongs to, from the profile
	// of the pane it opened from; empty for a folder of the OS Snow runs on.
	Distro string
	// Changed tells the server the pane's state changed, from any
	// goroutine. The server sends State to the pane's clients.
	Changed func()
	// Apply runs f on the model goroutine, after the functions applied
	// before it, then sends State to the pane's clients. A kind's own
	// goroutine hands its results to the pane this way, so the pane's state
	// needs no lock. It must not be called on the model goroutine.
	Apply func(f func())
}

// sideKind is a pane kind that opens a side pane. Open runs on the model
// goroutine and must not block.
type sideKind struct {
	Name, Title string
	Open        func(sideContext) (sidePane, error)
}

// paneKinds is the registry of the kinds of pane a server opens: terminal
// and chat, which are built in, and the side kinds.
type paneKinds struct {
	list []api.PaneKindInfo // in registration order, for a picker
	side map[string]sideKind
}

// sideKinds returns Snow's own side kinds, with each of extra in place of
// the one of its name, or added after them.
func sideKinds(extra []sideKind) []sideKind {
	kinds := []sideKind{gitKind()}
	for _, x := range extra {
		i := slices.IndexFunc(kinds, func(k sideKind) bool { return k.Name == x.Name })
		if i < 0 {
			kinds = append(kinds, x)
		} else {
			kinds[i] = x
		}
	}
	return kinds
}

// newPaneKinds returns the registry with terminal, chat when chats is set,
// and side.
func newPaneKinds(chats bool, side ...sideKind) (*paneKinds, error) {
	k := &paneKinds{list: []api.PaneKindInfo{{Name: api.PaneTerminal, Title: "Terminal"}}, side: map[string]sideKind{}}
	if chats {
		k.list = append(k.list, api.PaneKindInfo{Name: api.PaneChat, Title: "Chat"})
	}
	for _, sk := range side {
		if err := k.register(sk); err != nil {
			return nil, err
		}
	}
	return k, nil
}

// register adds side kind sk.
func (k *paneKinds) register(sk sideKind) error {
	if err := api.CheckKind(sk.Name); err != nil {
		return err
	}
	if sk.Open == nil {
		return fmt.Errorf("pane kind %q has no Open", sk.Name)
	}
	for _, have := range k.list {
		if have.Name == sk.Name {
			return fmt.Errorf("%w: %s", ErrDuplicateKind, sk.Name)
		}
	}
	k.list = append(k.list, api.PaneKindInfo{Name: sk.Name, Title: sk.Title})
	k.side[sk.Name] = sk
	return nil
}

// kinds returns the registered kinds, in a stable order.
func (k *paneKinds) kinds() []api.PaneKindInfo {
	return append([]api.PaneKindInfo(nil), k.list...)
}

// openSide opens side pane id of kind in folder, which belongs to the
// distribution of profile, the profile of the pane it opened from. It
// returns ErrUnknownKind for a kind that is not registered.
func (s *Server) openSide(id int, kind, profile, folder string) (*pane, error) {
	sk, ok := s.kinds.side[kind]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKind, kind)
	}
	distro, _ := s.distroOf(profile)
	sp, err := sk.Open(sideContext{
		Pane: id, Folder: folder, Distro: distro,
		Changed: func() { s.post(func(st *state) { st.sideChanged(id) }) },
		Apply: func(f func()) {
			s.send(func(st *state) {
				f()
				st.sideChanged(id)
			})
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open %s pane: %w", kind, err)
	}
	if sp == nil {
		return nil, fmt.Errorf("open %s pane: the kind opened nothing", kind)
	}
	return &pane{id: id, dir: folder, side: sp, sideKind: kind, sideRev: 1, sideFrom: profile}, nil
}

// sideChanged moves side pane id to its next state revision, which the next
// frame sends to its clients.
func (st *state) sideChanged(id int) {
	for _, sess := range st.sessions {
		if _, p := sess.findPane(id); p != nil && p.side != nil {
			p.sideRev++
			return
		}
	}
}

// syncSides sends c the state of each side pane of its session that it has
// not seen at the pane's current revision.
func (st *state) syncSides(c *client) {
	sess, ok := st.sessions[c.session]
	if !ok || c.dropped || len(c.out) >= cap(c.out)/2 {
		return
	}
	for _, w := range sess.windows {
		for id, p := range w.panes {
			if p.side == nil || c.sideRev[id] == p.sideRev {
				continue
			}
			// The revision counts as seen even when it cannot be sent, so a
			// bad state is logged once, not on every frame.
			c.sideRev[id] = p.sideRev
			state := p.side.State()
			if state == nil || state.Kind() != p.sideKind {
				st.srv.log.Warn("side pane state", "pane", id, "kind", p.sideKind, "err", "no state of its kind")
				continue
			}
			msg, err := api.NewSidePaneState(id, p.sideRev, state)
			if err != nil {
				st.srv.log.Warn("side pane state", "pane", id, "kind", p.sideKind, "err", err)
				continue
			}
			st.enqueue(c, msg)
		}
	}
}

// sideAction does a's action in the side pane of c's session it names.
func (st *state) sideAction(c *client, a *api.SidePaneAction) {
	if err := a.Validate(); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeBadCommand, Message: err.Error()})
		return
	}
	p := st.paneOf(c, a.Pane)
	if p == nil || p.side == nil {
		st.enqueue(c, &api.Error{Code: api.CodeBadCommand, Message: fmt.Sprintf("no side pane %d", a.Pane)})
		return
	}
	if err := p.side.Act(a.Action, a.Args); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: err.Error()})
		return
	}
	p.sideRev++
}
