// Package viewer reads a file for the viewer pane and marks the lines the
// conversation's recorded edits still explain. It reads a bounded amount
// and says what it left out.
package viewer

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Bounds on a read. A file past them shows its start and a limit.
const (
	MaxBytes  = 1 << 20
	MaxLines  = 2000
	MaxLine   = 500
	binaryLen = 8000
)

// Doc is the readable part of one file.
type Doc struct {
	Lines     []string // each at most MaxLine characters
	Binary    bool
	Truncated bool
	Limit     string // what was left out, in words, or ""
	Size      int64  // the file's size in bytes
}

// Read reads the file at path, up to MaxBytes, and says what it left out.
func Read(path string) (Doc, error) {
	f, err := os.Open(path)
	if err != nil {
		return Doc{}, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return Doc{}, err
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return Doc{}, err
	}
	return Parse(data, st.Size()), nil
}

// Parse builds a Doc from data, which may hold one byte past MaxBytes, and
// the file's whole size.
func Parse(data []byte, size int64) Doc {
	d := Doc{Size: size}
	if i := bytes.IndexByte(data[:min(len(data), binaryLen)], 0); i >= 0 {
		d.Binary = true
		d.Limit = "Binary file, not shown."
		return d
	}
	if len(data) > MaxBytes {
		d.Truncated = true
		data = data[:MaxBytes]
		d.Limit = fmt.Sprintf("Showing the first %s of %s.", sizeText(MaxBytes), sizeText(size))
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return d
	}
	if len(lines) > MaxLines {
		lines = lines[:MaxLines]
		d.Truncated = true
		if d.Limit == "" {
			d.Limit = fmt.Sprintf("Showing the first %d lines.", MaxLines)
		}
	}
	cut := false
	for i, line := range lines {
		if utf8.RuneCountInString(line) > MaxLine {
			r := []rune(line)
			lines[i] = string(r[:MaxLine])
			cut = true
		}
	}
	if cut {
		d.Truncated = true
		if d.Limit == "" {
			d.Limit = "Long lines are cut."
		} else if !strings.Contains(d.Limit, "Long lines") {
			d.Limit += " Long lines are cut."
		}
	}
	d.Lines = lines
	return d
}

// sizeText writes a byte count in the unit a person reads.
func sizeText(n int64) string {
	const k = 1024
	switch {
	case n >= k*k*k:
		return fmt.Sprintf("%.1f GiB", float64(n)/(k*k*k))
	case n >= k*k:
		return fmt.Sprintf("%.1f MiB", float64(n)/(k*k))
	case n >= k:
		return fmt.Sprintf("%.0f KiB", float64(n)/k)
	}
	return fmt.Sprintf("%d bytes", n)
}
