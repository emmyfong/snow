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
func keyLine(data string, key []string) int {
	var table []string
	var multiline byte
	best, bestLen := 0, 0
	for i, line := range strings.Split(data, "\n") {
		if multiline != 0 {
			multiline = multilineState(line, multiline)
			continue
		}
		// Continuation lines can start a multiline string even though they do
		// not contain a key. Scan before parsing so skipped array lines still
		// keep fake table headers inside the string from being treated as keys.
		multiline = multilineState(line, 0)
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

func multilineState(line string, quote byte) byte {
	for i := 0; i < len(line); {
		if quote != 0 {
			if quote == '"' && line[i] == '\\' {
				i += 2
				continue
			}
			if line[i] == quote {
				end := i
				for end < len(line) && line[end] == quote {
					end++
				}
				if end-i >= 3 {
					quote = 0
					i = end
					continue
				}
			}
			i++
			continue
		}
		if line[i] == '#' {
			break
		}
		if i+2 < len(line) && (line[i:i+3] == `"""` || line[i:i+3] == `'''`) {
			quote = line[i]
			i += 3
			continue
		}
		if line[i] == '"' || line[i] == '\'' {
			end := i + 1
			for end < len(line) {
				if line[i] == '"' && line[end] == '\\' {
					end += 2
					continue
				}
				if line[end] == line[i] {
					end++
					break
				}
				end++
			}
			i = end
			continue
		}
		i++
	}
	return quote
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
