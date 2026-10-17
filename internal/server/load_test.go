package server

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// The load budget from #47: with at least ten panes printing as fast as
// they can, typed input echoes within echoBudget (worst within worstEcho),
// and every pane keeps near the frame rate of 30 per second.
const (
	echoBudget = 50 * time.Millisecond
	worstEcho  = 250 * time.Millisecond
	minRate    = 24
	rateWindow = 3 * time.Second
)

// streamCommand prints without end, as fast as the shell can.
func streamCommand() string {
	if runtime.GOOS == "windows" {
		return "for /l %i in (0,0,1) do @echo snow-load\r"
	}
	return "yes snow-load\r"
}

// loadSession starts a session laid out as a 3 by 4 grid of panes, each
// about 79 by 17 cells: a quiet probe pane top left, and eleven panes that
// stream output. It returns the client, the probe, and the streams.
func loadSession(t testing.TB) (*apitest.Client, int, []int) {
	t.Helper()
	s := newTestServer(t)
	c := connect(t, s, 240, 72)
	c.Send(&api.Attach{Session: "load", Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	probe := l.Windows[0].Panes[0].ID
	n := 1
	splitNewest := func(target int, dir string) int {
		c.Send(split(target, dir))
		n++
		l = nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows[0].Panes) == n })
		return slices.MaxFunc(l.Windows[0].Panes, func(a, b api.PaneInfo) int { return a.ID - b.ID }).ID
	}
	columns := []int{probe}
	for range 2 {
		columns = append(columns, splitNewest(columns[len(columns)-1], api.DirRight))
	}
	c.Send(&api.Command{Op: api.OpEven, Target: probe})
	for _, top := range columns {
		last := top
		for range 3 {
			last = splitNewest(last, api.DirDown)
		}
		c.Send(&api.Command{Op: api.OpEven, Target: top})
	}
	l = nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows[0].Panes) == 12 })
	var ids []int
	for _, p := range l.Windows[0].Panes {
		ids = append(ids, p.ID)
	}
	waitPrompts(t, c, ids)
	var stream []int
	for _, id := range ids {
		if id != probe {
			stream = append(stream, id)
			c.Send(&api.Input{Pane: id, Paste: streamCommand()})
		}
	}
	return c, probe, stream
}

// waitPrompts reads updates until every pane in ids has shown a prompt:
// shells drop input that arrives while they start.
func waitPrompts(t testing.TB, c *apitest.Client, ids []int) {
	t.Helper()
	ready := map[int]bool{}
	want := strings.TrimRight(prompt(), " ")
	deadline := time.Now().Add(apitest.Wait)
	for len(ready) < len(ids) && time.Now().Before(deadline) {
		m, _ := c.Next(time.Second)
		if u, ok := m.(*api.PaneUpdate); ok {
			for _, l := range u.Lines {
				if strings.Contains(l.Text, want) {
					ready[u.Pane] = true
				}
			}
		}
	}
	if len(ready) < len(ids) {
		t.Fatalf("%d of %d panes showed a prompt", len(ready), len(ids))
	}
}

// probeEchoes types n unique markers into the probe pane, one at a time,
// and returns how long each took to come back in a PaneUpdate while the
// other panes stream, sorted.
func probeEchoes(t testing.TB, c *apitest.Client, probe, n int) []time.Duration {
	t.Helper()
	var echoes []time.Duration
	for i := range n {
		marker := fmt.Sprintf("probe%03d", i)
		sent := time.Now()
		c.Send(&api.Input{Pane: probe, Paste: marker})
		for seen := false; !seen; {
			m, err := c.Next(5 * time.Second)
			if err != nil {
				t.Fatalf("probe %s never echoed: %v", marker, err)
			}
			u, ok := m.(*api.PaneUpdate)
			if !ok || u.Pane != probe {
				continue
			}
			for _, l := range u.Lines {
				if strings.Contains(l.Text, marker) {
					seen = true
				}
			}
		}
		echoes = append(echoes, time.Since(sent))
		// Run the line, so the next marker starts on a clean prompt.
		c.Send(&api.Input{Pane: probe, Key: &api.Key{Code: api.KeyEnter}})
	}
	slices.Sort(echoes)
	return echoes
}

// slowestRate counts updates for rateWindow and returns the updates per
// second of the stream pane that got the fewest.
func slowestRate(c *apitest.Client, stream []int) float64 {
	// Updates queued before the window would count as if they came in it.
	for {
		if _, err := c.Next(time.Millisecond); err != nil {
			break
		}
	}
	counts := map[int]int{}
	end := time.Now().Add(rateWindow)
	for time.Now().Before(end) {
		if m, err := c.Next(time.Until(end)); err == nil {
			if u, ok := m.(*api.PaneUpdate); ok {
				counts[u.Pane]++
			}
		}
	}
	slowest := -1
	for _, id := range stream {
		if n := counts[id]; slowest < 0 || n < slowest {
			slowest = n
		}
	}
	return float64(slowest) / rateWindow.Seconds()
}

// TestInputLatencyUnderLoad: with eleven panes streaming, typed input still
// echoes within the budget, and every pane keeps updating. The budget is
// checked on Linux, where #47 sets it; elsewhere the numbers are only
// reported.
func TestInputLatencyUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("load test")
	}
	if raceDetector {
		t.Skip("timing means nothing under the race detector; CI runs this test without it")
	}
	c, probe, stream := loadSession(t)
	time.Sleep(500 * time.Millisecond) // let the streams reach full speed
	echoes := probeEchoes(t, c, probe, 20)
	median, worst := echoes[len(echoes)/2], echoes[len(echoes)-1]
	rate := slowestRate(c, stream)
	t.Logf("echo median %v, worst %v; slowest of %d streaming panes %.1f updates/s", median, worst, len(stream), rate)
	if runtime.GOOS != "linux" {
		return
	}
	if median >= echoBudget || worst >= worstEcho {
		t.Errorf("echo median %v, worst %v; want under %v and %v", median, worst, echoBudget, worstEcho)
	}
	if rate < minRate {
		t.Errorf("slowest streaming pane %.1f updates/s, want at least %d", rate, minRate)
	}
}

// BenchmarkTenStreams measures, with eleven panes streaming: input echo
// time, the slowest pane's update rate, and the heap. Run it with
//
//	go test -run '^$' -bench TenStreams ./internal/server
func BenchmarkTenStreams(b *testing.B) {
	c, probe, stream := loadSession(b)
	time.Sleep(500 * time.Millisecond)
	b.ResetTimer()
	echoes := probeEchoes(b, c, probe, b.N)
	b.StopTimer()
	rate := slowestRate(c, stream)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	b.ReportMetric(float64(echoes[len(echoes)/2].Microseconds())/1000, "echo-ms")
	b.ReportMetric(float64(echoes[len(echoes)-1].Microseconds())/1000, "worst-echo-ms")
	b.ReportMetric(rate, "updates/s")
	b.ReportMetric(float64(mem.HeapAlloc)/(1<<20), "heap-MB")
}
