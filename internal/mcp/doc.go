// Package mcp offers Snow's terminal panes to Claude Code as an MCP server
// over stdio. Claude Code starts snow mcp inside a pane, and every tool call
// asks the Snow server for the panes of that pane's session. The server
// speaks JSON-RPC 2.0, writes only MCP messages to stdout, and logs on
// stderr.
package mcp
