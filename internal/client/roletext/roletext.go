// Package roletext says what an agent role's limits are, and which of them
// Claude Code enforces. A role's instructions only ask; its permission
// mode is enforced. The new agent form and the agents board share it, so
// no screen calls an instruction-only role read-only.
package roletext

// Limits describes the limits of a role in permission mode, read-only when
// that mode keeps the agent from editing and running commands.
func Limits(mode string, readOnly bool) string {
	switch {
	case readOnly:
		return "read-only: " + mode + " mode, enforced"
	case mode != "":
		return mode + " mode, enforced"
	}
	return "instructions only, not enforced"
}

// Short is Limits in a word or two, for a list row.
func Short(mode string, readOnly bool) string {
	switch {
	case readOnly:
		return "read-only"
	case mode != "":
		return mode
	}
	return "not enforced"
}
