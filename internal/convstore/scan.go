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

// readLines decodes every non-blank line of the file at path. It has no
// limit on line length. A line that does not decode is returned with its
// error; only a failure to read the file stops it.
func readLines(path string) ([]line, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var lines []line
	r := bufio.NewReader(f)
	for n := 1; ; n++ {
		b, err := r.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read line %d: %w", n, err)
		}
		if t := bytes.TrimSpace(b); len(t) > 0 {
			rec, decErr := Decode(t)
			lines = append(lines, line{n: n, rec: rec, err: decErr})
		}
		if err != nil {
			return lines, nil
		}
	}
}
