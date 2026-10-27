// Package term runs programs on pseudo-terminals and keeps their screens.
// It wraps charmbracelet/x/xpty and charmbracelet/x/vt so that no other Snow
// package depends on those experimental APIs.
package term

// Spec describes the program a pane runs.
type Spec struct {
	Path string   // program to run, looked up in PATH if not absolute
	Args []string // arguments, without the program name
	Env  []string // KEY=value pairs added to the inherited environment; later wins
	Dir  string   // working directory; empty means the current one
	// CmdLine, when set, is the whole command line on Windows, passed to
	// the program as it is instead of joined from Path and Args: cmd reads
	// its own quoting. Other systems ignore it.
	CmdLine string
}

// DefaultScrollback is the number of lines a pane keeps above its screen.
// vt stores about 128 bytes per cell, so 10,000 lines can cost hundreds of MiB.
const DefaultScrollback = 2000

// Options tunes a pane. The zero value uses the defaults.
type Options struct {
	Scrollback int // lines kept above the screen; 0 means DefaultScrollback
	// Preamble is shown in the pane before the program's first output, as
	// if the program had printed it: for example the lines of a restored
	// session. It may hold ANSI styles.
	Preamble string
}

func (o Options) scrollback() int {
	if o.Scrollback > 0 {
		return o.Scrollback
	}
	return DefaultScrollback
}
