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

// The load budget from #47: with ten panes printing as fast as they can,
// typed input echoes within echoBudget, and every pane keeps near the frame
// rate (30 per second; minRate leaves room for a slow runner).
const (
	streams    = 10
	echoBudget = 50 * time.Millisecond
	minRate    = 25
)

// streamCommand prints without end, as fast as the shell can.
func streamCommand() string {
	if runtime.GOOS == "windows" {
		return "for /l %i in (0,0,1) do @echo snow-load\r"
	}
	return "yes snow-load\r"
}

// loadSession starts a session with one quiet probe pane and ten panes that
// stream output, and returns the client and the probe pane's ID.
func loadSession(t testing.TB) (*apitest.Client, int, []int) {
	t.Helper()
	s := newTestServer(t)
	c := connect(t, s, 240, 72)
	c.Send(&api.Attach{Session: "load", Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	probe := l.Windows[0].Panes[0].ID
	last := probe
	for i := range streams {
		dir := api.DirRight
		if i%2 == 1 {
			dir = api.DirDown
		}
		c.Send(split(last, dir))
		l = nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows[0].Panes) == i+2 })
		last = slices.MaxFunc(l.Windows[0].Panes, func(a, b api.PaneInfo) int { return a.ID - b.ID }).ID
	}
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

// loadResult is what one run of probes measured.
type loadResult struct {
	echoes  []time.Duration // per probe, sorted
	updates map[int]int     // pane ID → updates received
	elapsed time.Duration
}

func (r loadResult) median() time.Duration { return r.echoes[len(r.echoes)/2] }
func (r loadResult) worst() time.Duration  { return r.echoes[len(r.echoes)-1] }

// updateRate is the slowest streaming pane's updates per second.
func (r loadResult) updateRate(stream []int) float64 {
	slowest := -1
	for _, id := range stream {
		if n := r.updates[id]; slowest < 0 || n < slowest {
			slowest = n
		}
	}
	return float64(slowest) / r.elapsed.Seconds()
}

// probeEchoes types n unique markers into the probe pane, one at a time,
// and measures how long each takes to come back in a PaneUpdate while the
// other panes stream. It counts every update it reads.
func probeEchoes(t testing.TB, c *apitest.Client, probe, n int) loadResult {
	t.Helper()
	// Updates queued during warm-up would count as if they came now.
	for {
		if _, err := c.Next(time.Millisecond); err != nil {
			break
		}
	}
	r := loadResult{updates: map[int]int{}}
	start := time.Now()
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
			if !ok {
				continue
			}
			r.updates[u.Pane]++
			if u.Pane != probe {
				continue
			}
			for _, l := range u.Lines {
				if strings.Contains(l.Text, marker) {
					seen = true
				}
			}
		}
		r.echoes = append(r.echoes, time.Since(sent))
		// Run the line, so the next marker starts on a clean prompt.
		c.Send(&api.Input{Pane: probe, Key: &api.Key{Code: api.KeyEnter}})
	}
	r.elapsed = time.Since(start)
	slices.Sort(r.echoes)
	return r
}

// TestInputLatencyUnderLoad: with ten panes streaming, typed input still
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
	r := probeEchoes(t, c, probe, 20)
	t.Logf("echo median %v, worst %v; slowest streaming pane %.1f updates/s", r.median(), r.worst(), r.updateRate(stream))
	if runtime.GOOS != "linux" {
		return
	}
	if r.median() >= echoBudget {
		t.Errorf("echo median %v with %d streams, want under %v", r.median(), streams, echoBudget)
	}
	if rate := r.updateRate(stream); rate < minRate {
		t.Errorf("slowest streaming pane %.1f updates/s, want at least %d", rate, minRate)
	}
}

// BenchmarkTenStreams measures, with ten panes streaming: input echo time,
// the slowest pane's update rate, and the heap. Run it with
//
//	go test -run '^$' -bench TenStreams ./internal/server
func BenchmarkTenStreams(b *testing.B) {
	c, probe, stream := loadSession(b)
	time.Sleep(500 * time.Millisecond)
	b.ResetTimer()
	r := probeEchoes(b, c, probe, b.N)
	b.StopTimer()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	b.ReportMetric(float64(r.median().Microseconds())/1000, "echo-ms")
	b.ReportMetric(float64(r.worst().Microseconds())/1000, "worst-echo-ms")
	b.ReportMetric(r.updateRate(stream), "updates/s")
	b.ReportMetric(float64(mem.HeapAlloc)/(1<<20), "heap-MB")
}
