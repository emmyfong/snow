// Command snow-spike is throwaway code for epic #49. It runs one shell in a
// Bubble Tea pane through xpty and vt, to prove the stack on Windows and
// Linux. Epic #50 replaces it with internal/term and deletes this folder.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	shell := flag.String("shell", defaultShell(), "program to run in the pane")
	flag.Parse()

	p, err := newPane(*shell, flag.Args(), 80, 24)
	if err != nil {
		fmt.Fprintln(os.Stderr, "snow-spike:", err)
		os.Exit(1)
	}
	defer p.close()

	if _, err := tea.NewProgram(p).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "snow-spike:", err)
		os.Exit(1)
	}
}
