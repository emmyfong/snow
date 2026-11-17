package chat

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// fileList is the chat folder's files, as the server listed them. A chat
// asks for them once, when its folder is known, and uses them for the @
// picker and for the paths a reply may hold.
type fileList struct {
	folder string // the folder the files belong to
	paths  []string
	set    map[string]bool // paths, for a quick look
	loaded bool            // the server answered for this folder
	cut    bool            // the server left files out past its limit
	err    string          // why the folder could not be read
	want   *api.FileListRequest

	sel       int    // the highlighted row of the picker
	query     string // the filter the selection belongs to
	dismissed bool   // the user closed the picker for this query with Esc
}

// syncFiles keeps the files in step with the conversation's folder. A new
// folder drops the old files and asks for the new ones, once. A folder
// whose read failed is asked for again on a later snapshot, in case the
// cause is gone, such as a WSL distribution that has started.
func (m *Model) syncFiles() {
	f := &m.files
	if m.snap.Folder == "" {
		return
	}
	if f.folder == m.snap.Folder {
		if !f.loaded && f.err != "" && f.want == nil {
			f.err, f.want = "", &api.FileListRequest{Pane: m.snap.Pane}
			m.invalidate()
		}
		return
	}
	*f = fileList{folder: m.snap.Folder, want: &api.FileListRequest{Pane: m.snap.Pane}}
	m.atOpen = false
	m.invalidate()
}

// FileRequest returns the request for the chat folder's files that the
// session should send, once, or nil.
func (m *Model) FileRequest() *api.FileListRequest {
	r := m.files.want
	m.files.want = nil
	return r
}

// Files takes the server's list. A list for another pane or folder is
// dropped, so a late answer never replaces the files of the folder on
// screen.
func (m *Model) Files(l *api.FileList) {
	f := &m.files
	if m.snap.Folder == "" || l.Pane != m.snap.Pane || l.Folder != m.snap.Folder || f.loaded {
		return
	}
	if l.Err != "" {
		f.err = l.Err
		m.atOpen = false
		m.invalidate()
		return
	}
	f.paths, f.cut, f.loaded = l.Files, l.Cut, true
	f.set = make(map[string]bool, len(l.Files))
	for _, p := range l.Files {
		f.set[p] = true
	}
	f.sel = min(f.sel, max(len(f.paths)-1, 0))
	m.invalidate()
}

// known reports whether the server listed p as a file of the folder.
func (m *Model) known(p string) bool {
	return m.files.loaded && m.files.set[p]
}

// linkSpans returns the runs of text that are known files of the folder.
// An empty folder, or files the server has not listed yet, link nothing.
func (m *Model) linkSpans(text string) []ui.TextSpan {
	if !m.files.loaded || m.snap.Folder == "" {
		return nil
	}
	var spans []ui.TextSpan
	at := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimSuffix(line, "\n")
		for _, t := range pathsIn(body, m.snap.Folder) {
			if m.known(t.Path) {
				spans = append(spans, ui.TextSpan{Start: at + t.At, Len: t.Len})
			}
		}
		at += len([]rune(line))
	}
	return spans
}

// pathAt returns the path in row at cell x, with its line, when x lands on
// one. A path the server listed is a link; a path it did not list is one
// only when it looks like a file, so prose with a slash does not catch a
// click.
func (m *Model) pathAt(x int, row string) (string, int, bool) {
	if !m.files.loaded || m.snap.Folder == "" || x < 0 {
		return "", 0, false
	}
	plain := ansi.Strip(row)
	at := len([]rune(ansi.Truncate(plain, x, "")))
	for _, t := range pathsIn(plain, m.snap.Folder) {
		if at < t.At || at >= t.At+t.Len {
			continue
		}
		if m.known(t.Path) || looksLikeFile(t.Path) {
			return t.Path, t.Line, true
		}
	}
	return "", 0, false
}

// looksLikeFile reports whether p names a file by its last segment. A path
// the server did not list still gets a note when clicked.
func looksLikeFile(p string) bool {
	base := p
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return strings.Contains(base, ".") && base != "." && base != ".."
}
