package claudecode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// ErrUnreadable means a settings file is not JSON of the shape Claude Code
// uses. Snow refuses to change it.
var ErrUnreadable = errors.New("claudecode: Claude Code's settings are not JSON Snow can read")

// hookTimeout is the seconds Claude Code waits for snow hook. snow hook
// gives up much sooner; this only bounds a stuck process.
const hookTimeout = 5

// toolEvents take a matcher: the tools they run for.
var toolEvents = []string{EventPreToolUse, EventPostToolUse}

// handler is one hook command in a settings file.
type handler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// settings is a settings file's top object and its hooks object.
type settings struct {
	top   object
	hooks object
	form  format
}

// readSettings parses data; empty data is an empty object.
func readSettings(data []byte) (settings, error) {
	form, data := detectFormat(data)
	if len(bytes.TrimSpace(data)) == 0 {
		return settings{form: form}, nil
	}
	top, err := parseObject(data)
	if err != nil {
		return settings{}, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	var hooks object
	if raw, ok := top.get("hooks"); ok {
		if hooks, err = parseObject(raw); err != nil {
			return settings{}, fmt.Errorf("%w: hooks: %w", ErrUnreadable, err)
		}
	}
	return settings{top: top, hooks: hooks, form: form}, nil
}

// write encodes s with hooks in place, or without hooks when it is empty.
func (s settings) write() ([]byte, error) {
	top := s.top
	if len(s.hooks) == 0 {
		top = top.without("hooks")
	} else {
		top = top.set("hooks", s.hooks.encode())
	}
	return s.form.indentJSON(top.encode())
}

// groups returns event's hook groups.
func (s settings) groups(event string) ([]json.RawMessage, error) {
	raw, ok := s.hooks.get(event)
	if !ok {
		return nil, nil
	}
	gs, err := parseArray(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: hooks.%s: %w", ErrUnreadable, event, err)
	}
	return gs, nil
}

// setGroups stores event's groups, dropping the event when none is left.
func (s *settings) setGroups(event string, gs []json.RawMessage) {
	if len(gs) == 0 {
		s.hooks = s.hooks.without(event)
		return
	}
	s.hooks = s.hooks.set(event, encodeArray(gs))
}

// snowHandlers returns the commands of Snow's handlers in event.
func (s settings) snowHandlers(event string) ([]string, error) {
	gs, err := s.groups(event)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, g := range gs {
		hs, err := groupHandlers(g)
		if err != nil {
			return nil, err
		}
		for _, h := range hs {
			var hd handler
			if json.Unmarshal(h, &hd) == nil && IsSnowCommand(hd.Command) {
				out = append(out, hd.Command)
			}
		}
	}
	return out, nil
}

// groupHandlers returns the handlers of one hook group.
func groupHandlers(g json.RawMessage) ([]json.RawMessage, error) {
	o, err := parseObject(g)
	if err != nil {
		return nil, fmt.Errorf("%w: hook group: %w", ErrUnreadable, err)
	}
	raw, ok := o.get("hooks")
	if !ok {
		return nil, nil
	}
	hs, err := parseArray(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: hook group: %w", ErrUnreadable, err)
	}
	return hs, nil
}

// removeSnow takes Snow's handlers out of event. A group left with no
// handler goes too; other groups keep their bytes.
func (s *settings) removeSnow(event string) (bool, error) {
	gs, err := s.groups(event)
	if err != nil {
		return false, err
	}
	removed := false
	var keep []json.RawMessage
	for _, g := range gs {
		hs, err := groupHandlers(g)
		if err != nil {
			return false, err
		}
		left := slices.DeleteFunc(slices.Clone(hs), func(h json.RawMessage) bool {
			var hd handler
			return json.Unmarshal(h, &hd) == nil && IsSnowCommand(hd.Command)
		})
		switch {
		case len(left) == len(hs):
			keep = append(keep, g)
		case len(left) > 0:
			o, _ := parseObject(g) // groupHandlers parsed it already
			keep = append(keep, o.set("hooks", encodeArray(left)).encode())
			removed = true
		default:
			removed = true
		}
	}
	if removed {
		s.setGroups(event, keep)
	}
	return removed, nil
}

// snowGroup is the hook group Snow adds to event.
func snowGroup(event, command string) json.RawMessage {
	h := object{
		{"type", quote("command")},
		{"command", quote(command)},
		{"timeout", json.RawMessage(fmt.Sprint(hookTimeout))},
	}
	var g object
	if slices.Contains(toolEvents, event) {
		g = append(g, member{"matcher", quote("*")})
	}
	return append(g, member{"hooks", encodeArray([]json.RawMessage{h.encode()})}).encode()
}

// AddHooks returns data, a Claude Code settings file, with Snow's hook,
// running command, on every event in HookEvents. It keeps every other
// setting and hook. A Snow hook with another command, such as an older
// path to snow, is replaced. changed is false, and out is data, when every
// event already has exactly this hook. A file Snow cannot read gives
// ErrUnreadable and no output.
func AddHooks(data []byte, command string) (out []byte, changed bool, err error) {
	s, err := readSettings(data)
	if err != nil {
		return nil, false, err
	}
	for _, event := range HookEvents {
		cmds, err := s.snowHandlers(event)
		if err != nil {
			return nil, false, err
		}
		if len(cmds) == 1 && cmds[0] == command {
			continue
		}
		if _, err := s.removeSnow(event); err != nil {
			return nil, false, err
		}
		gs, _ := s.groups(event) // removeSnow read them already
		s.setGroups(event, append(gs, snowGroup(event, command)))
		changed = true
	}
	if !changed {
		return data, false, nil
	}
	out, err = s.write()
	return out, true, err
}

// RemoveHooks returns data without Snow's hooks. Every other setting and
// hook stays. changed is false, and out is data, when there was none.
func RemoveHooks(data []byte) (out []byte, changed bool, err error) {
	s, err := readSettings(data)
	if err != nil {
		return nil, false, err
	}
	for _, event := range HookEvents {
		removed, err := s.removeSnow(event)
		if err != nil {
			return nil, false, err
		}
		changed = changed || removed
	}
	if !changed {
		return data, false, nil
	}
	out, err = s.write()
	return out, true, err
}

// Status says which of Snow's hooks a settings file has.
type Status struct {
	Missing []string // events in HookEvents with no Snow hook
}

// Set reports whether every event has Snow's hook.
func (s Status) Set() bool { return len(s.Missing) == 0 }

// None reports whether no event has Snow's hook.
func (s Status) None() bool { return len(s.Missing) == len(HookEvents) }

// HookStatus reads data and says which events lack Snow's hook.
func HookStatus(data []byte) (Status, error) {
	s, err := readSettings(data)
	if err != nil {
		return Status{}, err
	}
	var st Status
	for _, event := range HookEvents {
		cmds, err := s.snowHandlers(event)
		if err != nil {
			return Status{}, err
		}
		if len(cmds) == 0 {
			st.Missing = append(st.Missing, event)
		}
	}
	return st, nil
}
