package shell

import "testing"

func TestDetectOnThisWindows(t *testing.T) {
	got := Detect()
	for _, p := range got {
		t.Logf("%-20s kind=%d path=%s args=%q", p.Name, p.Kind, p.Path, p.Args)
	}
	for _, p := range got {
		if p.Kind == KindCmd {
			return
		}
	}
	t.Fatal("cmd.exe not detected; every Windows machine has it")
}
