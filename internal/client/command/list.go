package command

import "github.com/emmyfong/snow/internal/client/ui"

// List is the rows a command list draws and the commands they come from.
type List struct {
	Commands []Command     // in the order the list shows them
	Items    []ui.ListItem // the rows, group headings included
	ItemOf   []int         // Items[i] names Commands[ItemOf[i]], or -1 for a heading
}

// NewList returns the rows for cmds. A grouped list sorts the commands by
// group and puts a heading before each group. describe adds the command's
// description column; a narrow pane leaves it out.
func NewList(cmds []Command, grouped, describe bool) List {
	if grouped {
		cmds = Grouped(cmds)
	}
	l := List{Commands: cmds, Items: make([]ui.ListItem, 0, len(cmds)), ItemOf: make([]int, 0, len(cmds))}
	prev, heading := Group(""), false
	for i, c := range cmds {
		if grouped && (!heading || c.Group != prev) {
			name := c.Group
			if name == "" {
				name = "Other"
			}
			l.Items = append(l.Items, ui.ListItem{Heading: string(name)})
			l.ItemOf = append(l.ItemOf, -1)
			prev, heading = c.Group, true
		}
		cols := []string{"/" + c.Line()}
		if describe {
			cols = append(cols, c.Desc)
		}
		l.Items = append(l.Items, ui.ListItem{Cols: cols})
		l.ItemOf = append(l.ItemOf, i)
	}
	return l
}

// Selected returns the row of the command at index cmd, or 0 when cmd is out
// of range.
func (l List) Selected(cmd int) int {
	for row, i := range l.ItemOf {
		if i == cmd {
			return row
		}
	}
	return 0
}
