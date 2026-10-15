// Package mouse turns a click position into what is under it: a pane, a
// border, a tab, or the rest of the status bar, and turns a border drag into
// resize commands. It is pure geometry: it knows nothing about Bubble Tea,
// so the session screen decides what each hit means.
package mouse
