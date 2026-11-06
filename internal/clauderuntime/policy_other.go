//go:build !windows && !darwin

package clauderuntime

// nativePolicyDir is where Claude Code reads managed settings on Linux.
const nativePolicyDir = "/etc/claude-code"
