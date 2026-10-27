package claudecode

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Setup adds Snow's hooks to t's settings. It shows the change and writes
// it only when the user answers yes on in. A file it cannot read is left
// alone, with ErrUnreadable.
func Setup(t Target, in *bufio.Reader, out io.Writer) error {
	return change(t, in, out, true)
}

// Remove takes Snow's hooks out of t's settings, after the user says yes.
func Remove(t Target, in *bufio.Reader, out io.Writer) error {
	return change(t, in, out, false)
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

// Report prints whether t has Snow's hooks.
func Report(t Target, out io.Writer) error {
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
