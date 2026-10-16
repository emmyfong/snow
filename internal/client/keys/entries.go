package keys

import (
	"strings"
)

// Entry is one row of which-key: the keys, written for people, and what
// they do.
type Entry struct {
	Keys string
	Desc string
}

// Entries lists the bound keys as which-key rows. Actions in one group share
// a row, so ten window keys take one line. Unbound actions have no row.
func (t *Table) Entries() []Entry {
	var out []Entry
	done := map[string]bool{}
	for _, a := range actions {
		if a.Group == "" {
			if ks := t.KeysOf(a.Name); len(ks) > 0 {
				out = append(out, Entry{Keys: joinKeys(ks), Desc: a.Desc})
			}
			continue
		}
		if done[a.Group] {
			continue
		}
		done[a.Group] = true
		var ks []string
		for _, m := range actions {
			if m.Group == a.Group {
				ks = append(ks, t.KeysOf(m.Name)...)
			}
		}
		if len(ks) > 0 {
			out = append(out, Entry{Keys: groupKeys(ks), Desc: groups[a.Group].desc})
		}
	}
	return out
}

func joinKeys(ks []string) string {
	shown := make([]string, len(ks))
	for i, k := range ks {
		shown[i] = DisplayKey(k)
	}
	return strings.Join(shown, "/")
}

// groupKeys shortens a group's keys: "0 1 ... 9" to "0-9", and the four
// arrows with one modifier to "Ctrl+Arrows".
func groupKeys(ks []string) string {
	if s, ok := digitRange(ks); ok {
		return s
	}
	if s, ok := arrowSet(ks); ok {
		return s
	}
	return joinKeys(ks)
}

func digitRange(ks []string) (string, bool) {
	if len(ks) < 3 {
		return "", false
	}
	for i, k := range ks {
		if len(k) != 1 || k[0] < '0' || k[0] > '9' || (i > 0 && k[0] != ks[i-1][0]+1) {
			return "", false
		}
	}
	return ks[0] + "-" + ks[len(ks)-1], true
}

func arrowSet(ks []string) (string, bool) {
	if len(ks) != 4 {
		return "", false
	}
	mod := ""
	for i, k := range ks {
		cut := strings.LastIndex(k, "-") + 1
		if i == 0 {
			mod = k[:cut]
		}
		switch k[cut:] {
		case "Up", "Down", "Left", "Right":
		default:
			return "", false
		}
		if k[:cut] != mod {
			return "", false
		}
	}
	seen := map[string]bool{}
	for _, k := range ks {
		seen[k] = true
	}
	if len(seen) != 4 {
		return "", false
	}
	return DisplayKey(mod + "Arrows"), true
}
