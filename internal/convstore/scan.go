package convstore

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// line is one non-blank line of a conversation file, decoded.
type line struct {
	n   int    // line number, from 1
	rec Record // nil when err is set
	err error
}

// eachLine calls fn with the number and bytes of every non-blank line of
// the file at path, without the newline. It has no limit on line length.
// fn must not keep b. Only a failure to read the file stops it.
func eachLine(path string, fn func(n int, b []byte)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	r := bufio.NewReaderSize(f, 64<<10)
	for n := 1; ; n++ {
		b, err := r.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			// A line longer than the buffer: collect it whole. b points into
			// the buffer, which the next read reuses, so copy it first.
			head := bytes.Clone(b)
			var rest []byte
			rest, err = r.ReadBytes('\n')
			b = append(head, rest...)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read line %d: %w", n, err)
		}
		if t := bytes.TrimSpace(b); len(t) > 0 {
			fn(n, t)
		}
		if err != nil {
			return nil
		}
	}
}

// readLines decodes every non-blank line of the file at path. A line that
// does not decode is returned with its error.
func readLines(path string) ([]line, error) {
	var lines []line
	err := eachLine(path, func(n int, b []byte) {
		rec, err := Decode(b)
		lines = append(lines, line{n: n, rec: rec, err: err})
	})
	return lines, err
}
