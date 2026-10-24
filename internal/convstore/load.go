package convstore

import (
	"errors"
	"fmt"
)

// Errors that Load and List wrap in a *LoadError.
var (
	ErrNewerVersion = errors.New("file is from a newer version of Snow")
	ErrNoMeta       = errors.New("file does not start with a meta record")
)

// LoadError reports a conversation file that cannot be loaded. The file is
// kept as it is, for the user.
type LoadError struct {
	Path string
	Err  error
}

func (e *LoadError) Error() string { return fmt.Sprintf("load conversation %s: %v", e.Path, e.Err) }

// Unwrap returns the underlying error.
func (e *LoadError) Unwrap() error { return e.Err }

// TypeDamaged is the type of a Damaged record.
const TypeDamaged Type = "damaged"

// Damaged marks the place of a line that Load could not read. Load makes
// it; it is never in a file.
type Damaged struct {
	Line int
	Err  error
}

// Type implements Record.
func (*Damaged) Type() Type { return TypeDamaged }

// Problem is a line that Load dropped or skipped.
type Problem struct {
	Line int
	// Torn is true for a last line cut off by a crash. Otherwise the line
	// is in the middle of the file and a Damaged record marks its place.
	Torn bool
	Err  error
}

func (p Problem) String() string {
	if p.Torn {
		return fmt.Sprintf("line %d: torn last line dropped: %v", p.Line, p.Err)
	}
	return fmt.Sprintf("line %d: damaged line skipped: %v", p.Line, p.Err)
}

// resultLost is the content of a tool result that Load adds for a tool call
// whose result was never saved.
const resultLost = "result lost"

// Conversation is a conversation file as Load read it.
type Conversation struct {
	Path string
	Meta Meta
	// Records follow the meta record, in file order, with Load's changes:
	// partial text becomes a cut-off Assistant, a lost tool result gets a
	// stand-in, and a damaged line becomes a Damaged record.
	Records  []Record
	Problems []Problem
	// MidTurn is true when the file ends in the middle of a turn: after a
	// user message, with no whole reply or interruption after it. The
	// caller shows the turn as interrupted; Load does not write the file.
	MidTurn bool
}

// Load reads the conversation id. It never writes, renames, or deletes the
// file. A file it cannot load gives a *LoadError; a bad id gives an error
// wrapping ErrBadID.
func (s *Store) Load(id string) (*Conversation, error) {
	path, err := s.Path(id)
	if err != nil {
		return nil, err
	}
	lines, err := readLines(path)
	if err != nil {
		return nil, &LoadError{Path: path, Err: err}
	}
	var meta *Meta
	if len(lines) > 0 {
		meta, _ = lines[0].rec.(*Meta)
	}
	if err := checkMeta(meta, id); err != nil {
		return nil, &LoadError{Path: path, Err: err}
	}
	c := &Conversation{Path: path, Meta: *meta}
	c.build(lines[1:])
	return c, nil
}

// checkMeta checks the meta record of the file of conversation id. A nil
// meta means the first line is not a meta record.
func checkMeta(meta *Meta, id string) error {
	switch {
	case meta == nil:
		return ErrNoMeta
	case meta.Version > Version:
		return fmt.Errorf("%w: version %d, this Snow reads up to %d", ErrNewerVersion, meta.Version, Version)
	case meta.ID != id:
		return fmt.Errorf("%w: meta record names %q", ErrBadID, meta.ID)
	}
	return nil
}

// build turns the lines after the meta record into Records, Problems, and
// MidTurn.
func (c *Conversation) build(lines []line) {
	var (
		partial *Partial // the latest partial text since the last reply
		calls   []string // tool calls of the last reply with no result yet
		open    bool     // a turn has started and not ended
	)
	// settle ends what a record other than a result or approval leaves
	// behind: partial text without its reply, and calls without results.
	settle := func() {
		if partial != nil {
			c.Records = append(c.Records, &Assistant{At: partial.At, CutOff: true,
				Blocks: []Block{{Kind: BlockText, Text: partial.Text}}})
			partial = nil
		}
		for _, id := range calls {
			c.Records = append(c.Records, &ToolResult{ToolUseID: id, Content: resultLost, IsError: true, Lost: true})
		}
		calls = nil
	}
	for i, l := range lines {
		if l.err != nil {
			if i == len(lines)-1 {
				c.Problems = append(c.Problems, Problem{Line: l.n, Torn: true, Err: l.err})
				continue
			}
			c.Problems = append(c.Problems, Problem{Line: l.n, Err: l.err})
			c.Records = append(c.Records, &Damaged{Line: l.n, Err: l.err})
			continue
		}
		switch r := l.rec.(type) {
		case *Partial:
			partial = r
			continue
		case *Assistant:
			partial = nil
			settle()
			c.Records = append(c.Records, r)
			calls = toolCalls(r)
			open = r.Stop == StopToolUse
			continue
		case *ToolResult:
			calls = remove(calls, r.ToolUseID)
		case *Approval:
		case *User:
			settle()
			open = true
		case *Interruption:
			settle()
			open = false
		default:
			settle()
		}
		c.Records = append(c.Records, l.rec)
	}
	if partial != nil || len(calls) > 0 {
		open = true
	}
	settle()
	c.MidTurn = open
}

// toolCalls returns the IDs of the tool calls in a reply.
func toolCalls(a *Assistant) []string {
	var ids []string
	for _, b := range a.Blocks {
		if b.Kind == BlockToolUse {
			ids = append(ids, b.ID)
		}
	}
	return ids
}

func remove(ids []string, id string) []string {
	for i, v := range ids {
		if v == id {
			return append(ids[:i:i], ids[i+1:]...)
		}
	}
	return ids
}
