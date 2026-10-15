// Package clipboard puts text on the system clipboard. On Windows it uses the
// native clipboard; elsewhere it asks the terminal Snow runs in, with an OSC 52
// escape sequence, and the terminal sets the clipboard. That also works over
// SSH, and from WSL into Windows Terminal.
package clipboard
