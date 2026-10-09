// Package term runs programs on pseudo-terminals and keeps their screens.
// It wraps charmbracelet/x/xpty and charmbracelet/x/vt so that no other Snow
// package depends on those experimental APIs.
package term

// Spec describes the program a pane runs.
type Spec struct {
	Path string   // program to run, looked up in PATH if not absolute
	Args []string // arguments, without the program name
	Env  []string // full environment; nil means the current process's
	Dir  string   // working directory; empty means the current one
}

// DefaultScrollback is the number of lines a pane keeps above its screen.
// vt stores about 128 bytes per cell, so 10,000 lines can cost hundreds of MiB.
const DefaultScrollback = 2000

// Options tunes a pane. The zero value uses the defaults.
type Options struct {
	Scrollback int // lines kept above the screen; 0 means DefaultScrollback
}

func (o Options) scrollback() int {
	if o.Scrollback > 0 {
		return o.Scrollback
	}
	return DefaultScrollback
}
