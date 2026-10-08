// Command snow is a terminal cockpit for directing AI coding agents.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, stdout io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		if _, err := fmt.Fprintf(stdout, "snow %s\n", version); err != nil {
			return 1
		}
		return 0
	}
	if _, err := fmt.Fprintln(stdout, "usage: snow version"); err != nil {
		return 1
	}
	return 2
}
