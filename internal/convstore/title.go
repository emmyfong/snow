package convstore

// Title returns the conversation's title as the history list shows it: the
// last rename, else the title in the meta record, else the start of the
// first message. A conversation with none has an empty title.
func (c *Conversation) Title() string {
	title, renamed, first := "", false, ""
	for _, r := range c.Records {
		switch r := r.(type) {
		case *User:
			if first == "" {
				first = titleFrom(r.Text)
			}
		case *Rename:
			title, renamed = r.Title, true
		}
	}
	switch {
	case renamed:
		return title
	case c.Meta.Title != "":
		return c.Meta.Title
	}
	return first
}
