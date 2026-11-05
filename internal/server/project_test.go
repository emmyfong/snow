package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

func TestProjectKey(t *testing.T) {
	tests := []struct {
		name string
		a, b [2]string // distro, path
		same bool
	}{
		{"windows case", [2]string{"", `C:\Src\Snow`}, [2]string{"", `c:\src\snow`}, true},
		{"windows slashes", [2]string{"", `C:\Src\Snow\`}, [2]string{"", `c:/src/snow`}, true},
		{"drive root", [2]string{"", `C:\`}, [2]string{"", `c:`}, false},
		{"wsl and its windows view", [2]string{"Ubuntu", "/home/u/snow"}, [2]string{"", `\\wsl.localhost\Ubuntu\home\u\snow`}, false},
		{"wsl case", [2]string{"Ubuntu", "/home/u/Snow"}, [2]string{"Ubuntu", "/home/u/snow"}, false},
		{"two distros", [2]string{"Ubuntu", "/home/u/snow"}, [2]string{"Debian", "/home/u/snow"}, false},
		{"unix case", [2]string{"", "/src/Snow"}, [2]string{"", "/src/snow"}, false},
		{"unix clean", [2]string{"", "/src/snow/"}, [2]string{"", "/src/./snow"}, true},
	}
	for _, tt := range tests {
		if got := normalForm(tt.a[0], tt.a[1]) == normalForm(tt.b[0], tt.b[1]); got != tt.same {
			t.Errorf("%s: %q vs %q same = %v, want %v", tt.name, normalForm(tt.a[0], tt.a[1]), normalForm(tt.b[0], tt.b[1]), got, tt.same)
		}
	}
}

// TestProjectKeyGitRoot: a folder inside a git repository has the
// repository's key; a folder outside one has its own.
func TestProjectKeyGitRoot(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "internal", "server")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if got, want := projectKey(tool.Folder{Path: sub}), normalForm("", repo); got != want {
		t.Fatalf("key %q, want the repository's %q", got, want)
	}
	plain := t.TempDir()
	if got, want := projectKey(tool.Folder{Path: plain}), normalForm("", plain); got != want {
		t.Fatalf("key %q, want the folder's %q", got, want)
	}
}

// projectServer is a chat server that saves project layouts in a new
// folder, with a short save delay.
func projectServer(t *testing.T) (*Server, *store.Projects) {
	t.Helper()
	opts, _, _ := chatOptions(t)
	opts.ProjectsDir = filepath.Join(t.TempDir(), "projects")
	opts.saveDelay = 20 * time.Millisecond
	s := New(opts)
	t.Cleanup(s.Close)
	p, err := store.OpenProjects(opts.ProjectsDir)
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}

// openIn attaches c to a new session in dir and returns its first layout
// and its chat pane.
func openIn(t *testing.T, c *apitest.Client, session, dir string) (*api.Layout, int) {
	t.Helper()
	c.Send(&api.Attach{Session: session, Create: true, Dir: dir})
	l := nextLayout(t, c, func(l *api.Layout) bool { return l.Session == session && chatIn(l) != 0 })
	return l, chatIn(l)
}

// chatFolderOf returns the folder the server gave chat pane id.
func chatFolderOf(t *testing.T, s *Server, id int) tool.Folder {
	t.Helper()
	var f tool.Folder
	s.call(func(st *state) {
		for _, sess := range st.sessions {
			if _, p := sess.findPane(id); p != nil && p.chat != nil {
				f = s.projectFolder(&restoredChat{folder: p.chat.folder, profile: p.chat.profile})
			}
		}
	})
	return f
}

// savedTemplate waits for the project of f to have a saved layout.
func savedTemplate(t *testing.T, p *store.Projects, f tool.Folder) layout.Template {
	t.Helper()
	var tmpl layout.Template
	eventually(t, "the project layout is saved", func() bool {
		raw, err := p.LoadLayout(projectKey(f))
		return err == nil && json.Unmarshal(raw, &tmpl) == nil
	})
	return tmpl
}

// TestRememberSaves: a layout change saves the chat window's layout for
// its project after the save delay.
func TestRememberSaves(t *testing.T) {
	dir := t.TempDir()
	s, p := projectServer(t)
	c := connect(t, s, 100, 30)
	_, chat := openIn(t, c, "one", dir)
	f := chatFolderOf(t, s, chat)
	if _, err := p.LoadLayout(projectKey(f)); err == nil {
		t.Fatal("a layout was saved before any change")
	}
	c.Send(&api.Command{Op: api.OpSplit, Target: chat, Args: map[string]string{api.ArgDir: api.DirLeft}})
	nextLayout(t, c, func(l *api.Layout) bool { return terminalIn(l) != 0 })
	tmpl := savedTemplate(t, p, f)
	if got := tmpl.Kinds(); !reflect.DeepEqual(got, []string{api.PaneTerminal, api.PaneChat}) {
		t.Fatalf("saved kinds %v, want terminal then chat", got)
	}
}

// TestRememberRestoresNewSession: a new session in the same folder starts
// with the saved layout: the same kinds, in the same places.
func TestRememberRestoresNewSession(t *testing.T) {
	dir := t.TempDir()
	s, p := projectServer(t)
	c := connect(t, s, 100, 30)
	_, chat := openIn(t, c, "one", dir)
	c.Send(&api.Command{Op: api.OpSplit, Target: chat, Args: map[string]string{api.ArgDir: api.DirDown}})
	nextLayout(t, c, func(l *api.Layout) bool { return terminalIn(l) != 0 })
	c.Send(&api.Command{Op: api.OpResize, Target: chat, Args: map[string]string{api.ArgDir: api.DirDown, api.ArgCells: "5"}})
	before := nextLayout(t, c, func(l *api.Layout) bool { ps := shownPanes(l); return len(ps) == 2 && ps[0].Rect.H > ps[1].Rect.H })
	savedTemplate(t, p, chatFolderOf(t, s, chat))
	eventually(t, "the resize is saved", func() bool {
		raw, _ := p.LoadLayout(projectKey(chatFolderOf(t, s, chat)))
		var tmpl layout.Template
		_ = json.Unmarshal(raw, &tmpl)
		return len(tmpl.Ratios) == 2 && tmpl.Ratios[0] > 0.6
	})

	c2 := connect(t, s, 100, 30)
	after, _ := openIn(t, c2, "two", dir)
	for i, want := range shownPanes(before) {
		got := shownPanes(after)[i]
		if got.Kind != want.Kind || got.Rect != want.Rect {
			t.Errorf("pane %d: %s %+v, want %s %+v", i, got.Kind, got.Rect, want.Kind, want.Rect)
		}
	}
}

// writeProject saves body as the layout of the project of dir.
func writeProject(t *testing.T, p *store.Projects, dir string, tmpl layout.Template) {
	t.Helper()
	b, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SaveLayout(projectKey(tool.Folder{Path: usableDir(dir)}), b); err != nil {
		t.Fatal(err)
	}
}

// TestRememberSkipsUnknownKind: a saved kind the server cannot open is
// left out, and the session still opens.
func TestRememberSkipsUnknownKind(t *testing.T) {
	dir := t.TempDir()
	s, p := projectServer(t)
	writeProject(t, p, dir, layout.Template{Axis: layout.Horizontal, Ratios: []float64{0.3, 0.7}, Children: []layout.Template{
		{Kind: "files"}, {Kind: api.PaneChat},
	}})
	c := connect(t, s, 100, 30)
	l, _ := openIn(t, c, "one", dir)
	if ps := shownPanes(l); len(ps) != 1 || ps[0].Kind != api.PaneChat {
		t.Fatalf("panes %+v, want the chat alone", ps)
	}
}

// TestRememberCorruptFile: a project file that cannot be read keeps its
// place on disk, and the session opens with the default layout.
func TestRememberCorruptFile(t *testing.T) {
	dir := t.TempDir()
	s, p := projectServer(t)
	writeProject(t, p, dir, layout.Template{Kind: api.PaneChat})
	entries, err := os.ReadDir(s.opts.ProjectsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("project files %v, %v", entries, err)
	}
	file := filepath.Join(s.opts.ProjectsDir, entries[0].Name())
	if err := os.WriteFile(file, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := connect(t, s, 100, 30)
	l, _ := openIn(t, c, "one", dir)
	if ps := shownPanes(l); len(ps) != 1 || ps[0].Kind != api.PaneChat {
		t.Fatalf("panes %+v, want the default chat", ps)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "{not json" {
		t.Fatalf("the file changed: %q, %v", b, err)
	}
}
