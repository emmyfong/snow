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
		o.Gauge = "ctx " + tokens(used) + " tok"
	}
	switch {
	case m.snap.Cost > 0 || u.Priced:
		o.Cost = dollars(m.snap.Cost) + " est."
	case u.Window > 0:
		o.Cost = tokens(used) + " tok"
	}
	return o
}

// tokens shows a token count briefly: 950, 12.3k, 1.2M.
func tokens(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprint(n)
	case n < 1_000_000:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1000)) + "k"
	default:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1_000_000)) + "M"
	}
}

func trimZero(s string) string {
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

// dollars shows a cost in US dollars to the cent; a cost below a cent
// shows as "<$0.01".
func dollars(c float64) string {
	if c > 0 && c < 0.005 {
		return "<$0.01"
	}
	return fmt.Sprintf("$%.2f", c)
}
