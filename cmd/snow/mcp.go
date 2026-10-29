package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/emmyfong/snow/internal/mcp"
	"github.com/emmyfong/snow/internal/transport"
)

// runMCP is snow mcp: the MCP server Claude Code starts inside a Snow pane.
// It reads SNOW_PANE and SNOW_SOCKET from the pane's environment, offers the
// pane tools, and stops when Claude Code closes its input. Outside a Snow
// pane it offers no tools and says so on stderr.
func runMCP(stdout, stderr io.Writer) int {
	err := mcp.Serve(context.Background(), stdin, stdout, mcp.Options{
		Version: version,
		Pane:    os.Getenv(transport.EnvPane),
		Socket:  os.Getenv(transport.EnvSocket),
		Log:     log.New(stderr, "", 0),
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	return 0
}
