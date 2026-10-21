// Package keys is the client's key binding table: which key, pressed after
// the prefix, runs which action. The table starts from tmux's defaults and
// applies the user's changes from the config file. Which-key, the right-click
// menu, and the session screen all read it, so each shows the keys the user
// really has. It also decides which key presses are text (Typed) and how a
// one-line text field is edited (Edit).
package keys
