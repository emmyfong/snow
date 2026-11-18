package server

import (
	"encoding/json"
	"path"
	"runtime"
	"strings"

	"github.com/emmyfong/snow/internal/viewer"
	"github.com/emmyfong/snow/pkg/api"
)

// windowEvidence returns the edit and write records of the window's chat
// that changed file, which is an absolute logical path. Records without a
// result, or whose result failed, are not evidence of a change.
func windowEvidence(w *window, file string) []viewer.Evidence {
	id, ok := w.paneOfKind(api.PaneChat)
	if !ok {
		return nil
	}
	p := w.panes[id]
	if p == nil || p.chat == nil {
		return nil
	}
	var out []viewer.Evidence
	for _, entry := range p.chat.snap.Entries {
		if entry.Kind != api.ChatEntryTool || entry.Tool == nil {
			continue
		}
		t := entry.Tool
		if !t.Done || t.IsError || t.Name != "edit" && t.Name != "write" {
			continue
		}
		switch t.Name {
		case "edit":
			var in struct {
				Path string `json:"path"`
				Old  string `json:"old"`
				New  string `json:"new"`
			}
			if json.Unmarshal(t.Input, &in) != nil || logicalAbs(p.chat.folder, in.Path) != file {
				continue
			}
			out = append(out, viewer.Evidence{Turn: entry.Turn, Tool: t.Name, Old: in.Old, New: in.New})
		case "write":
			var in struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if json.Unmarshal(t.Input, &in) != nil || logicalAbs(p.chat.folder, in.Path) != file {
				continue
			}
			out = append(out, viewer.Evidence{Turn: entry.Turn, Tool: t.Name, New: in.Content})
		}
	}
	return out
}

// logicalAbs returns file, or p below folder, as a clean slash path so a
// tool's path and a pane's path compare.
func logicalAbs(folder, p string) string {
	folder, p = slashPath(folder), slashPath(p)
	if path.IsAbs(p) || len(p) > 1 && p[1] == ':' {
		return path.Clean(p)
	}
	return path.Clean(path.Join(folder, p))
}

// slashPath turns a Windows path's separators into slashes. A WSL path
// stays as it is: a backslash is a name character in Linux.
func slashPath(p string) string {
	if runtime.GOOS == "windows" && !strings.HasPrefix(p, "/") {
		return strings.ReplaceAll(p, `\`, "/")
	}
	return p
}
