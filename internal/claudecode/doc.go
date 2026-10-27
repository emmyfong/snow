// Package claudecode connects Snow to Claude Code through Claude Code's
// documented surfaces only: its hooks and its settings file. snow hook
// turns a hook event into the agent state of the pane Claude Code runs in,
// and snow claude setup adds that hook to Claude Code's settings.
//
// Snow never reads, stores, or forwards Claude Code's login or tokens.
// This package touches no credential and no other setting.
package claudecode
