package claudecode

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Setup adds Snow's hooks to t's settings and registers Snow's MCP server
// with m, after the user says yes to each. A program m cannot start is
// reported, and the hooks are still set.
func Setup(t Target, m MCP, in *bufio.Reader, out io.Writer) error {
	if err := change(t, in, out, true); err != nil {
		return err
	}
	return registerMCP(m, in, out)
}

// Remove takes Snow's hooks out of t's settings and unregisters Snow's MCP
// server, after the user says yes to each.
func Remove(t Target, m MCP, in *bufio.Reader, out io.Writer) error {
	if err := change(t, in, out, false); err != nil {
		return err
	}
	return unregisterMCP(m, in, out)
}

func change(t Target, in *bufio.Reader, out io.Writer, add bool) error {
	data, err := t.Read()
	if err != nil {
		return err
	}
	edit := RemoveHooks
	if add {
		edit = func(data []byte) ([]byte, bool, error) { return AddHooks(data, t.Command()) }
	}
	next, changed, err := edit(data)
	if err != nil {
		return fmt.Errorf("%s: %w; nothing changed", t.Name(), err)
	}
	if !changed {
		if add {
			_, err = fmt.Fprintf(out, "%s: Snow's hooks are already set.\n", t.Name())
		} else {
			_, err = fmt.Fprintf(out, "%s: no Snow hooks to remove.\n", t.Name())
		}
		return err
	}
	if err := describe(out, t, data, next, add); err != nil {
		return err
	}
	if !yes(in) {
		_, err := fmt.Fprintln(out, "Nothing changed.")
		return err
	}
	now, err := t.Read()
	if err != nil {
		return err
	}
	if (now == nil) != (data == nil) || !bytes.Equal(now, data) {
		return fmt.Errorf("%s: %w", t.Name(), ErrChanged)
	}
	if err := t.Write(next); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Done. Claude Code sessions use the change when they start again.")
	return err
}

// describe shows the change, the summary then the exact diff, and asks.
func describe(out io.Writer, t Target, data, next []byte, add bool) error {
	var b strings.Builder
	if add {
		fmt.Fprintf(&b, "Snow adds this hook to %s:\n  %s\n", t.Name(), t.Command())
		fmt.Fprintf(&b, "for the events %s.\n", strings.Join(HookEvents, ", "))
		b.WriteString("It tells Snow which Claude Code pane works, waits for you, or is idle.\n")
		b.WriteString("Every other setting and hook stays. Snow never reads your Claude Code login.\n")
	} else {
		fmt.Fprintf(&b, "Snow removes its hooks from %s. Every other setting and hook stays.\n", t.Name())
	}
	if data != nil {
		fmt.Fprintf(&b, "A copy of the file goes to %s first, unless one is there already.\n", t.Backup())
	}
	b.WriteString("The change:\n")
	b.WriteString(unifiedDiff(data, next, t.Name(), t.Name()))
	b.WriteString("Write the change? [y/N] ")
	_, err := io.WriteString(out, b.String())
	return err
}

// yes reads one answer line and reports whether it is y or yes.
func yes(in *bufio.Reader) bool {
	line, _ := in.ReadString('\n') // no answer, or a read error, is a no
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// Report prints whether t has Snow's hooks and whether m has Snow's MCP
// server.
func Report(t Target, m MCP, out io.Writer) error {
	if err := reportHooks(t, out); err != nil {
		return err
	}
	return reportMCP(m, out)
}

// reportHooks prints whether t has Snow's hooks.
func reportHooks(t Target, out io.Writer) error {
	data, err := t.Read()
	if err != nil {
		return err
	}
	st, err := HookStatus(data)
	switch {
	case err != nil:
		_, err = fmt.Fprintf(out, "%s: %v\n", t.Name(), err)
	case st.Set():
		_, err = fmt.Fprintf(out, "%s: Snow's hooks are set.\n", t.Name())
	case st.None():
		_, err = fmt.Fprintf(out, "%s: not set. Run snow claude setup.\n", t.Name())
	default:
		_, err = fmt.Fprintf(out, "%s: partly set; missing %s. Run snow claude setup.\n", t.Name(), strings.Join(st.Missing, ", "))
	}
	return err
}

// registerMCP adds Snow's MCP server to m's Claude Code, after the user says
// yes. A Claude Code Snow cannot start is reported, not treated as a
// failure: the hooks work without it.
func registerMCP(m MCP, in *bufio.Reader, out io.Writer) error {
	if !m.Installed() {
		_, err := fmt.Fprintf(out, "%s: Claude Code is not on PATH, so Snow did not add its MCP server.\n", m.displayName())
		return err
	}
	set, err := m.Status()
	if err != nil {
		return err
	}
	if set {
		_, err = fmt.Fprintf(out, "%s: Snow's MCP server is already set.\n", m.displayName())
		return err
	}
	if err := describeMCP(out, m, true); err != nil {
		return err
	}
	if !yes(in) {
		_, err := fmt.Fprintln(out, "Nothing changed.")
		return err
	}
	if err := m.Add(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Done. Claude Code sessions see Snow's panes when they start again.")
	return err
}

// unregisterMCP removes Snow's MCP server from m's Claude Code, after the
// user says yes.
func unregisterMCP(m MCP, in *bufio.Reader, out io.Writer) error {
	if !m.Installed() {
		_, err := fmt.Fprintf(out, "%s: Claude Code is not on PATH, so Snow has no MCP server to remove.\n", m.displayName())
		return err
	}
	set, err := m.Status()
	if err != nil {
		return err
	}
	if !set {
		_, err = fmt.Fprintf(out, "%s: no Snow MCP server to remove.\n", m.displayName())
		return err
	}
	if err := describeMCP(out, m, false); err != nil {
		return err
	}
	if !yes(in) {
		_, err := fmt.Fprintln(out, "Nothing changed.")
		return err
	}
	if err := m.Remove(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Done. Claude Code sessions no longer see Snow's panes.")
	return err
}

// describeMCP shows the MCP change and asks.
func describeMCP(out io.Writer, m MCP, add bool) error {
	var b strings.Builder
	if add {
		fmt.Fprintf(&b, "Snow registers its MCP server with %s:\n  %s\n", m.displayName(), m.Command())
		b.WriteString("It lets a Claude Code agent list and read the terminal panes of its Snow session,\n")
		b.WriteString("and type into one after you approve it in Claude Code's own prompt.\n")
		b.WriteString("Claude Code writes its own user configuration; Snow never edits it or reads your login.\n")
		b.WriteString("Register it? [y/N] ")
	} else {
		fmt.Fprintf(&b, "Snow removes its MCP server from %s. Claude Code keeps its other servers.\n", m.displayName())
		b.WriteString("Remove it? [y/N] ")
	}
	_, err := io.WriteString(out, b.String())
	return err
}

// reportMCP prints whether m has Snow's MCP server.
func reportMCP(m MCP, out io.Writer) error {
	if !m.Installed() {
		_, err := fmt.Fprintf(out, "%s: Claude Code is not on PATH, so Snow has no MCP server to report.\n", m.displayName())
		return err
	}
	set, err := m.Status()
	if err != nil {
		return err
	}
	if set {
		_, err = fmt.Fprintf(out, "%s: Snow's MCP server is set.\n", m.displayName())
		return err
	}
	_, err = fmt.Fprintf(out, "%s: Snow's MCP server is not set. Run snow claude setup.\n", m.displayName())
	return err
}
