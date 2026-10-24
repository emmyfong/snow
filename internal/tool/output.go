package tool

import (
	"fmt"
	"strings"
)

// capped collects tool output up to a limit in bytes, so a big file or a
// broad search cannot flood the model's context.
type capped struct {
	b     strings.Builder
	limit int
	full  bool
}

// add appends s and its newline. It reports false once the limit is hit;
// the caller then stops and says what it cut.
func (c *capped) add(s string) bool {
	if c.full || c.b.Len()+len(s)+1 > c.limit {
		c.full = true
		return false
	}
	c.b.WriteString(s)
	c.b.WriteByte('\n')
	return true
}

// String returns the output, ending with notice when the limit was hit.
func (c *capped) String(notice string) string {
	if c.full && notice != "" {
		return c.b.String() + fmt.Sprintf("[cut: %s]\n", notice)
	}
	return c.b.String()
}

// input decodes a tool's JSON input into v.
func input(raw []byte, v any) error {
	if err := jsonUnmarshal(raw, v); err != nil {
		return fmt.Errorf("bad input: %w", err)
	}
	return nil
}
