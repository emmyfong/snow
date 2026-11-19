package chat

import (
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// pathStops end a path in text: spaces and the punctuation that surrounds
// a path in prose. A colon stays in the token, for a :line suffix.
const pathStops = " \t\n\"'`()[]{}<>,;|"

// pathToken is one path in a line of text, and where it sits.
type pathToken struct {
	Path string // from the chat's folder, with forward slashes
	Line int    // the one-based line to reveal, or 0
	At   int    // the token's first rune in the line
	Len  int    // the token's runes
}

// pathsIn returns the paths in text that lie below folder, with where each
// sits. A token is a path when it has a slash, has no URL scheme, and does
// not climb out of the folder; a bare file name such as "main.go" is not,
// because prose is full of dots. A trailing :line names a line in the file.
func pathsIn(text, folder string) []pathToken {
	if text == "" || folder == "" {
		return nil
	}
	root := filepath.ToSlash(folder)
	runes := []rune(text)
	var out []pathToken
	for i := 0; i < len(runes); {
		if strings.ContainsRune(pathStops, runes[i]) {
			i++
			continue
		}
		start := i
		for i < len(runes) && !strings.ContainsRune(pathStops, runes[i]) {
			i++
		}
		tok := trimPath(string(runes[start:i]))
		if p, line, ok := relativePath(tok, root); ok {
			out = append(out, pathToken{Path: p, Line: line, At: start, Len: len([]rune(tok))})
		}
	}
	return out
}

// trimPath drops the punctuation that ends a path in prose. A colon stays:
// it may start the line number.
func trimPath(tok string) string {
	return strings.TrimRight(tok, ".,;!?")
}

// relativePath turns tok, a token from text, into a clean path below root.
func relativePath(tok, root string) (string, int, bool) {
	if tok == "" || strings.Contains(tok, "://") {
		return "", 0, false
	}
	line := 0
	p := tok
	p = strings.TrimSuffix(p, ":")
	if i := strings.LastIndexByte(p, ':'); i > 0 && i+1 < len(p) && allDigits(p[i+1:]) {
		lineSuffix := p[i+1:]
		p = p[:i]
		if j := strings.LastIndexByte(p, ':'); j > 0 && j+1 < len(p) && allDigits(p[j+1:]) {
			lineSuffix = p[j+1:]
			p = p[:j]
		}
		n, err := strconv.Atoi(lineSuffix)
		if err != nil || n < 1 {
			return "", 0, false
		}
		line = n
	}
	p = filepath.ToSlash(p)
	if _, ok := drivePath(p); ok {
		rel, ok := underRoot(p, root, true)
		return rel, line, ok
	}
	if strings.Contains(p, ":") || strings.HasPrefix(p, "/") {
		rel, ok := underRoot(p, root, false)
		return rel, line, ok
	}
	if strings.HasSuffix(p, "/") || !strings.Contains(p, "/") {
		return "", 0, false
	}
	// A host name with a path, such as github.com/snow/snow, is not a file
	// below the folder. A first part that starts with a dot is a folder,
	// such as .github.
	if first, _, _ := strings.Cut(p, "/"); !strings.HasPrefix(first, ".") && strings.Contains(first, ".") {
		return "", 0, false
	}
	rel := path.Clean(p)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", 0, false
	}
	return rel, line, true
}

// underRoot returns p as a path below root, and whether it is one. A
// Windows path compares without case; the result keeps the text's case.
func underRoot(p, root string, fold bool) (string, bool) {
	if root == "" || len(p) <= len(root) {
		return "", false
	}
	head := p[:len(root)]
	if fold {
		if !strings.EqualFold(head, root) {
			return "", false
		}
	} else if head != root {
		return "", false
	}
	if p[len(root)] != '/' {
		return "", false
	}
	rel := path.Clean(strings.TrimPrefix(p[len(root):], "/"))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

// drivePath reports whether p starts with a Windows drive letter, such as
// C:/Users, and its letter in lower case.
func drivePath(p string) (string, bool) {
	if len(p) < 3 || p[1] != ':' || p[2] != '/' {
		return "", false
	}
	c := p[0]
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return strings.ToLower(string(c)), true
	}
	return "", false
}

// allDigits reports whether s is one or more ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
