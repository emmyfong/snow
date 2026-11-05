package server

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/pkg/api"
)

// ErrUnknownPreset is returned for a layout name not in api.LayoutPresets.
var ErrUnknownPreset = errors.New("unknown layout")

// presets are the Templates of api.LayoutPresets. Files, viewer, and git
// are side kinds that later epics register; until then a preset skips them.
var presets = map[string]layout.Template{
	"ide": {Axis: layout.Horizontal, Ratios: []float64{0.2, 0.6, 0.2}, Children: []layout.Template{
		{Kind: "files"},
		{Axis: layout.Vertical, Ratios: []float64{0.4, 0.4, 0.2}, Children: []layout.Template{
			{Kind: "viewer"}, {Kind: api.PaneChat}, {Kind: api.PaneTerminal},
		}},
		{Kind: "git"},
	}},
	"focus":  {Kind: api.PaneChat},
	"review": {Axis: layout.Horizontal, Ratios: []float64{0.5, 0.5}, Children: []layout.Template{{Kind: "viewer"}, {Kind: api.PaneChat}}},
}

// newLeaf is a pane a layout plan opens: its reserved ID and its kind.
type newLeaf struct {
	id   int
	kind string
}

// layoutPlan is what applying a preset to a window does, worked out
// before anything changes.
type layoutPlan struct {
	tree    *layout.Tree
	open    []newLeaf // panes to open, with IDs from st.nextPane up
	moved   []int     // panes of the window the preset does not hold
	skipped []string  // kinds the server cannot open
}

// planLayout works out preset name for window w of sess. It changes
// nothing. Existing panes fill the preset's leaves of their kind, lowest
// ID first and plain terminals before agent ones, so a running chat and
// its shells keep going.
func (st *state) planLayout(sess *session, w *window, name string) (layoutPlan, error) {
	tmpl, ok := presets[name]
	if !ok {
		names := make([]string, len(api.LayoutPresets))
		for i, p := range api.LayoutPresets {
			names[i] = p.Name
		}
		return layoutPlan{}, fmt.Errorf("%w %q. The layouts are: %s", ErrUnknownPreset, name, strings.Join(names, ", "))
	}
	var plan layoutPlan
	tmpl, ok = tmpl.Prune(func(kind string) bool {
		if st.srv.kinds.has(kind) {
			return true
		}
		if !slices.Contains(plan.skipped, kind) {
			plan.skipped = append(plan.skipped, kind)
		}
		return false
	})
	if !ok {
		return layoutPlan{}, fmt.Errorf("this server cannot open any pane of the %s layout", name)
	}

	byKind := map[string][]*pane{}
	for _, p := range w.panes {
		byKind[p.kind()] = append(byKind[p.kind()], p)
	}
	for _, ps := range byKind {
		slices.SortFunc(ps, func(a, b *pane) int {
			if (a.agent != "") != (b.agent != "") {
				if a.agent == "" {
					return -1
				}
				return 1
			}
			return cmp.Compare(a.id, b.id)
		})
	}
	next := st.nextPane
	var ids []layout.PaneID
	for _, kind := range tmpl.Kinds() {
		if ps := byKind[kind]; len(ps) > 0 {
			ids = append(ids, layout.PaneID(ps[0].id))
			byKind[kind] = ps[1:]
			continue
		}
		plan.open = append(plan.open, newLeaf{id: next, kind: kind})
		ids = append(ids, layout.PaneID(next))
		next++
	}
	for _, ps := range byKind {
		for _, p := range ps {
			plan.moved = append(plan.moved, p.id)
		}
	}
	slices.Sort(plan.moved)
	cols, rows := sess.area()
	tree, err := tmpl.Build(ids, cols, rows)
	if err != nil {
		return layoutPlan{}, err
	}
	plan.tree = tree
	return plan, nil
}

// applyLayout applies preset cmd.Args[ArgName] to the window holding pane
// cmd.Target, as planLayout plans it. New panes take the target pane's
// profile and folder. Panes the preset does not hold move to a new window
// named "moved", which is not shown; nothing closes. On an error the
// window is unchanged. notice names the kinds the preset skipped.
func (st *state) applyLayout(sess *session, cmd *api.Command) (notice string, err error) {
	w, target := sess.findPane(cmd.Target)
	if target == nil {
		return "", fmt.Errorf("no pane %d", cmd.Target)
	}
	name := cmd.Args[api.ArgName]
	plan, err := st.planLayout(sess, w, name)
	if err != nil {
		return "", err
	}
	profile, dir := st.newPaneArgs(sess, cmd, target.id)
	cols, rows := sess.area()
	rects := layout.Rects(plan.tree, cols, rows).Panes
	opened := map[int]*pane{}
	for _, leaf := range plan.open {
		r := rects[layout.PaneID(leaf.id)]
		p, err := st.openPane(leaf.id, r.W, r.H, leaf.kind, profile, dir)
		if err != nil {
			for _, o := range opened {
				st.closePane(o)
			}
			return "", fmt.Errorf("open %s pane: %w", leaf.kind, err)
		}
		opened[leaf.id] = p
	}
	st.nextPane += len(plan.open)

	if len(plan.moved) > 0 {
		st.moveToWindow(sess, w, plan.moved, "moved")
	}
	for id, p := range opened {
		w.panes[id] = p
	}
	w.tree = plan.tree
	if _, kept := w.panes[w.focused]; !kept {
		w.focused = int(plan.tree.Panes()[0])
		if id, ok := w.paneOfKind(api.PaneChat); ok {
			w.focused = id
		}
	}
	if len(plan.skipped) > 0 {
		notice = fmt.Sprintf("Snow cannot open these panes yet, so the %s layout leaves them out: %s.", name, strings.Join(plan.skipped, ", "))
	}
	return notice, nil
}

// moveToWindow takes panes ids out of window w and puts them side by side
// in a new window named name, at the lowest free index. The shown window
// stays shown. The caller replaces w's tree.
func (st *state) moveToWindow(sess *session, w *window, ids []int, name string) {
	nw := &window{index: sess.freeIndex(), name: name, tree: layout.New(layout.PaneID(ids[0])), panes: map[int]*pane{}, focused: ids[0]}
	for i, id := range ids {
		nw.panes[id] = w.panes[id]
		delete(w.panes, id)
		if i > 0 {
			// Split cannot fail here: the previous pane is in the tree and
			// id is not.
			_ = nw.tree.Split(layout.PaneID(ids[i-1]), layout.Right, layout.PaneID(id))
		}
	}
	if len(ids) > 1 {
		_ = nw.tree.Even(layout.PaneID(ids[0]))
	}
	sess.windows = append(sess.windows, nw)
	slices.SortFunc(sess.windows, func(a, b *window) int { return a.index - b.index })
}
