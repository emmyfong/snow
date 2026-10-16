package keys

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/config"
)

// Binding is one key and the action it runs after the prefix.
type Binding struct {
	Key    string // canonical, for example "C-Left"
	Action Action
}

// Table maps keys to actions. It is read-only after New, so the screen, the
// popup, and the menu may share one.
type Table struct {
	prefix string            // canonical
	order  []string          // keys in listing order
	action map[string]Action // by canonical key
}

// Default returns the table of tmux's keys with the prefix Ctrl+b.
func Default() *Table {
	t, err := New(config.Defaults().Keys)
	if err != nil {
		// The defaults are fixed and checked by tests.
		panic(fmt.Sprintf("keys: default table is invalid: %v", err))
	}
	return t
}

// New builds the table from the defaults and the user's changes. Typing the
// prefix again sends it to the pane, so the prefix key is bound to
// send-prefix unless the user bound it to something else. It returns an
// *Error for a key or action that does not exist.
func New(cfg config.Keys) (*Table, error) {
	prefix, err := ParseKey(cfg.Prefix)
	if err != nil {
		return nil, &Error{Setting: "keys.prefix", Key: cfg.Prefix, Err: err}
	}
	t := &Table{prefix: prefix, action: map[string]Action{}}
	for _, b := range defaults {
		t.set(b.key, b.action)
	}
	// Map order is random; sorting makes the listing, and so which key a
	// menu shows, the same on every run.
	userKeys := make([]string, 0, len(cfg.Bind))
	for k := range cfg.Bind {
		userKeys = append(userKeys, k)
	}
	slices.Sort(userKeys)
	written := map[string]string{} // canonical key to the spelling that set it
	user := map[string]bool{}      // keys the user bound to a real action
	for _, raw := range userKeys {
		key, err := ParseKey(raw)
		if err != nil {
			return nil, &Error{Setting: "keys.bind", Key: raw, Action: cfg.Bind[raw], Err: err}
		}
		if other, dup := written[key]; dup {
			return nil, &Error{Setting: "keys.bind", Key: raw, Action: cfg.Bind[raw],
				Err: fmt.Errorf("%w: %q and %q are both %s", ErrDuplicateKey, other, raw, key)}
		}
		written[key] = raw
		name := cfg.Bind[raw]
		if name == config.UnbindAction {
			t.unset(key)
			continue
		}
		if _, ok := ActionByName(name); !ok {
			return nil, &Error{Setting: "keys.bind", Key: raw, Action: name,
				Err: fmt.Errorf("%w %q", ErrUnknownAction, name)}
		}
		t.set(key, name)
		user[key] = true
	}
	if a, bound := t.action[prefix]; bound && !user[prefix] {
		return nil, &Error{Setting: "keys.prefix", Key: cfg.Prefix, Err: fmt.Errorf(
			"%w: %s is also bound to %s; unbind it or bind it elsewhere under [keys.bind]", ErrPrefixShadows, prefix, a.Name)}
	}
	if _, bound := t.action[prefix]; !bound {
		t.set(prefix, SendPrefix)
	}
	return t, nil
}

func (t *Table) set(key, name string) {
	a, _ := ActionByName(name)
	if _, ok := t.action[key]; !ok {
		t.order = append(t.order, key)
	}
	t.action[key] = a
}

func (t *Table) unset(key string) {
	delete(t.action, key)
	t.order = slices.DeleteFunc(t.order, func(k string) bool { return k == key })
}

// IsPrefix reports whether k is the prefix key.
func (t *Table) IsPrefix(k tea.Key) bool { return Name(k) == t.prefix }

// PrefixLabel writes the prefix for people, for example "Ctrl+b".
func (t *Table) PrefixLabel() string { return DisplayKey(t.prefix) }

// Lookup returns the action bound to the key press k.
func (t *Table) Lookup(k tea.Key) (Action, bool) { return t.Action(Name(k)) }

// Action returns the action bound to a canonical key.
func (t *Table) Action(key string) (Action, bool) {
	a, ok := t.action[key]
	return a, ok
}

// Bindings lists every binding in table order: the defaults first, then the
// user's other keys by name.
func (t *Table) Bindings() []Binding {
	out := make([]Binding, 0, len(t.order))
	for _, k := range t.order {
		out = append(out, Binding{Key: k, Action: t.action[k]})
	}
	return out
}

// KeysOf returns the canonical keys bound to the action called name, in
// table order.
func (t *Table) KeysOf(name string) []string {
	var out []string
	for _, k := range t.order {
		if t.action[k].Name == name {
			out = append(out, k)
		}
	}
	return out
}

// Shortcut writes how to run the action by keyboard, for example
// "Ctrl+b %". It names the action's first key, and is "" when no key is
// bound to the action.
func (t *Table) Shortcut(name string) string {
	ks := t.KeysOf(name)
	if len(ks) == 0 {
		return ""
	}
	return t.PrefixLabel() + " " + DisplayKey(ks[0])
}
