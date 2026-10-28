package shell

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestStartBackgroundStopsTree: Stop ends the program and the processes
// it started.
func TestStartBackgroundStopsTree(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	line := `while true; do echo tick; sleep 0.2; done > ` + out
	if runtime.GOOS == "windows" {
		line = `ping -n 60 127.0.0.1 > ` + out
	}
	p, err := StartBackground(local(t).Command(dir, line), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	waitGrow(t, out, 0)
	select {
	case <-p.Done():
		t.Fatal("the program exited by itself")
	default:
	}
	if err := p.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("Stop returned before the program exited")
	}
	before := size(out)
	time.Sleep(2500 * time.Millisecond)
	if after := size(out); after != before {
		t.Fatalf("the child still writes: %d bytes, then %d", before, after)
	}
}

// TestStartBackgroundExit: a program that exits closes Done, with its
// output in out and its failure in Err.
func TestStartBackgroundExit(t *testing.T) {
	var out lockedWriter
	p, err := StartBackground(local(t).Command(t.TempDir(), "echo boom && exit 3"), &out)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the program did not exit")
	}
	if p.Err() == nil || !bytes.Contains(out.bytes(), []byte("boom")) {
		t.Fatalf("Err %v, output %q; want a failure and boom", p.Err(), out.bytes())
	}
	if err := p.Stop(time.Second); err != nil {
		t.Fatalf("Stop after the exit: %v", err)
	}
}
