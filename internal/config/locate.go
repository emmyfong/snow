package config

import (
	"slices"
	"strconv"
	"strings"
)

// keyLine returns the line, from 1, where key is defined in a TOML document:
// its table header or its key/value line. The TOML library does not expose
// key positions, so this scans the text. When key itself has no line of its
// own (it sits in an inline table), keyLine returns the line of its deepest
// parent that has one. It returns 0 when no part of key is found.
//
// The scan does not track multi-line strings, so a line inside one that
// looks like a key can match. A wrong line there is harmless: the error
// still names the key.
func keyLine(data string, key []string) int {
	var table []string
	best, bestLen := 0, 0
	for i, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		var path []string
		switch {
		case line == "" || line[0] == '#':
			continue
		case line[0] == '[':
			parts, rest := splitKey(strings.TrimLeft(line, "["))
			if parts == nil || !strings.HasPrefix(rest, "]") {
				continue
			}
			table, path = parts, parts
		default:
			parts, rest := splitKey(line)
			if parts == nil || !strings.HasPrefix(rest, "=") {
				continue
			}
			path = append(slices.Clone(table), parts...)
		}
		if len(path) > len(key) || !slices.Equal(path, key[:len(path)]) {
			continue
		}
		if len(path) == len(key) {
			return i + 1
		}
		if len(path) > bestLen {
			best, bestLen = i+1, len(path)
		}
	}
	return best
}

// splitKey reads a dotted TOML key, such as `a."b.c".d`, from the start of
// s. It returns the key's parts and the text after the key with leading
// blanks removed. It returns nil parts when s does not start with a key.
func splitKey(s string) (parts []string, rest string) {
	for {
		s = strings.TrimLeft(s, " \t")
		var part string
		switch {
		case strings.HasPrefix(s, `"`):
			end := closingQuote(s)
			if end < 0 {
				return nil, ""
			}
			p, err := strconv.Unquote(s[:end+1])
			if err != nil {
				return nil, ""
			}
			part, s = p, s[end+1:]
		case strings.HasPrefix(s, "'"):
			end := strings.IndexByte(s[1:], '\'')
			if end < 0 {
				return nil, ""
			}
			part, s = s[1:end+1], s[end+2:]
		default:
			end := strings.IndexFunc(s, func(r rune) bool { return !isBareKeyChar(r) })
			if end < 0 {
				end = len(s)
			}
			if end == 0 {
				return nil, ""
			}
			part, s = s[:end], s[end:]
		}
		parts = append(parts, part)
		s = strings.TrimLeft(s, " \t")
		if !strings.HasPrefix(s, ".") {
			return parts, s
		}
		s = s[1:]
	}
}

// closingQuote returns the index of the quote that ends the basic string at
// the start of s, skipping escaped characters, or -1 if there is none.
func closingQuote(s string) int {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}

func isBareKeyChar(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-'
}
