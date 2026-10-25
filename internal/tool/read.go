package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Limits of the read tool.
const (
	readLines     = 2000      // lines returned when the call gives no limit
	readBytes     = 256 << 10 // output cap
	readLineChars = 2000      // a longer line is cut
)

type readTool struct{}

func init() { register(readTool{}) }

func (readTool) Def() Def {
	return Def{
		Name: "read",
		Description: "Read a text file. Each line starts with its number. For a long file, " +
			"read a part with offset (the first line, from 1) and limit (the number of lines).",
		Schema: schema(map[string]any{
			"path":   prop("string", "the file, relative to the conversation folder or absolute"),
			"offset": prop("integer", "the first line to read, from 1"),
			"limit":  prop("integer", "how many lines to read"),
		}, "path"),
	}
}

func (readTool) Run(ctx context.Context, env Env, raw json.RawMessage) (string, error) {
	var in struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := input(raw, &in); err != nil {
		return "", err
	}
	p, err := env.Folder.Resolve(in.Path)
	if err != nil {
		return "", err
	}
	first, limit := max(in.Offset, 1), in.Limit
	if limit <= 0 {
		limit = readLines
	}
	// Check before opening: opening a named pipe blocks until a writer
	// comes, and a cancel cannot stop it.
	info, err := os.Stat(p.OS)
	switch {
	case err != nil:
		return "", fmt.Errorf("read %s: %w", in.Path, err)
	case info.IsDir():
		return "", fmt.Errorf("%s is a folder; use list", in.Path)
	case !info.Mode().IsRegular():
		return "", fmt.Errorf("%s is not a regular file", in.Path)
	}
	f, err := os.Open(p.OS)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", in.Path, err)
	}
	defer func() { _ = f.Close() }()
	r := bufio.NewReader(f)
	if head, _ := r.Peek(8000); isBinary(head) {
		return "", fmt.Errorf("%s is a binary file", in.Path)
	}
	out := capped{limit: readBytes}
	n, last := 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		line, err := readLine(r, readLineChars)
		if line != "" {
			n++
			if n >= first && n < first+limit && !out.full {
				if out.add(fmt.Sprintf("%6d\t%s", n, cutLine(strings.TrimRight(line, "\r\n"), readLineChars))) {
					last = n
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read %s: %w", in.Path, err)
		}
	}
	switch {
	case n == 0:
		return "(empty file)\n", nil
	case first > n:
		return "", fmt.Errorf("%s has %d lines; offset %d is past the end", in.Path, n, first)
	case last < n || out.full:
		out.full = true
		return out.String(fmt.Sprintf("showing lines %d-%d of %d; read more with offset %d", first, last, n, last+1)), nil
	}
	return out.String(""), nil
}

// isBinary reports whether the start of a file holds a NUL byte, as git
// decides.
func isBinary(head []byte) bool { return strings.IndexByte(string(head), 0) >= 0 }

// cutLine shortens a line to at most n bytes, on a character boundary.
func cutLine(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n] + "…"
}

// Paths implements Pather.
func (readTool) Paths(input json.RawMessage) ([]string, error) { return pathField(input, "") }

// readLine reads one line, with its newline, keeping at most limit+4 bytes
// of it, so a huge file with no newline cannot fill memory. cutLine then
// marks the line as cut. The rest of a long line is read and dropped.
func readLine(r *bufio.Reader, limit int) (string, error) {
	var b []byte
	for {
		chunk, err := r.ReadSlice('\n')
		if room := limit + 4 - len(b); room > 0 {
			b = append(b, chunk[:min(room, len(chunk))]...)
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return string(b), err
		}
	}
}
