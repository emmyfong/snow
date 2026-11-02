package chat

import (
	"fmt"
	"math"

	"github.com/emmyfong/snow/internal/client/ui"
)

// hotGauge is the share of the context window, in percent, from which the
// gauge warns.
const hotGauge = 80

// minBarHeight is the smallest pane that gets a top bar; a smaller one
// keeps its rows for the transcript and the input.
const minBarHeight = 4

// hasBar reports whether the pane shows the top bar: once its snapshot
// came, when it is tall enough.
func (m *Model) hasBar() bool { return m.loaded && m.height >= minBarHeight }

// barOptions are what the top bar shows: the title, how full the context
// was on the last reply, the estimated cost, the model, and the mode.
// Without a price the cost becomes a token count; without a context
// window the gauge does.
func (m *Model) barOptions() ui.TopBarOptions {
	o := ui.TopBarOptions{Width: m.width, Title: m.snap.Title, Model: m.snap.Model}
	if o.Model == "" {
		o.Model = "no model"
	}
	if m.snap.Effort != "" {
		o.Model += " " + m.snap.Effort
	}
	if m.snap.Mode != "" {
		o.Mode = "mode: " + m.snap.Mode
	}
	u := m.snap.Usage
	if !m.usage || u == nil {
		return o
	}
	used := u.Input + u.CacheRead + u.CacheWrite + u.Output
	if u.Window > 0 {
		pct := int(math.Round(float64(used) * 100 / float64(u.Window)))
		o.Gauge, o.GaugeHot = fmt.Sprintf("ctx %d%%", pct), pct >= hotGauge
	} else {
		o.Gauge = "ctx " + ui.ShortCount(used) + " tok"
	}
	switch {
	case m.snap.Cost > 0 || u.Priced:
		o.Cost = ui.Dollars(m.snap.Cost) + " est."
	case u.Window > 0:
		o.Cost = ui.ShortCount(used) + " tok"
	}
	return o
}

// modelSpanAt reports whether x is on the top bar's model.
func modelSpanAt(spans []ui.TopBarSpan, x int) bool {
	for _, s := range spans {
		if s.Kind == ui.TopBarModel && x >= s.X && x < s.X+s.W {
			return true
		}
	}
	return false
}
