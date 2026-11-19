package ui

// Tab is one window in the window bar.
type Tab struct {
	Index  int
	Name   string
	Active bool
	Zoomed bool
	Agent  Mark // the most urgent state of the window's agents
	Driven bool // an agent types into one of its panes
}
