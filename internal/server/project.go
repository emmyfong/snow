package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// A project remembers the layout of the window that holds its chat, so a
// new session in the same project starts with it. The project is the git
// repository above the chat's folder, or the folder itself.

// openProjects opens opts.ProjectsDir and its saver, or returns nils when
// project layouts are off or the folder cannot be made.
func (s *Server) openProjects() (*store.Projects, *store.ProjectSaver) {
	if s.opts.ProjectsDir == "" {
		return nil, nil
	}
	p, err := store.OpenProjects(s.opts.ProjectsDir)
	if err != nil {
		s.log.Warn("project layouts will not be saved", "err", err)
		return nil, nil
	}
	delay := orDefault(s.opts.saveDelay, defaultSaveDelay)
	return p, store.NewProjectSaver(p, delay, func(key string, err error) {
		s.log.Warn("save project layout", "project", key, "err", err)
	})
}

// projectKey returns the key of the project that holds f: the nearest
// folder at or above f with a .git entry, or f, in normalForm. It reads the
// disk, so it never runs on the model goroutine.
func projectKey(f tool.Folder) string {
	dir, up := f.Path, filepath.Dir
	join := filepath.Join
	if f.Distro != "" {
		up, join = path.Dir, path.Join
	}
	root := f.Path
	for {
		if p, err := f.Resolve(join(dir, ".git")); err == nil {
			if _, err := os.Stat(p.OS); err == nil {
				root = dir
				break
			}
		}
		parent := up(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return normalForm(f.Distro, root)
}

// normalForm is the project key of folder p, in distro or on this OS. A
// Windows path compares without case and with either slash. A WSL folder
// keeps its distro and its case, so it differs from its \\wsl.localhost
// view.
func normalForm(distro, p string) string {
	if distro != "" {
		return "wsl:" + distro + ":" + path.Clean(strings.ReplaceAll(p, `\`, "/"))
	}
	if !windowsPath(p) {
		return path.Clean(p)
	}
	p = strings.ToLower(strings.ReplaceAll(p, "/", `\`))
	if trimmed := strings.TrimRight(p, `\`); len(trimmed) > 2 || !strings.HasSuffix(trimmed, ":") {
		p = trimmed
	}
	return p
}

// windowsPath reports whether p is a Windows path: a drive letter or a UNC
// path. It does not depend on the OS Snow runs on, so tests see one rule.
func windowsPath(p string) bool {
	if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
		return true
	}
	return len(p) >= 2 && p[1] == ':' && (p[0]|0x20 >= 'a' && p[0]|0x20 <= 'z')
}

// projectLayout returns the layout saved for the project of chat folder f,
// or nil when there is none or it cannot be used. A file that cannot be
// read stays on disk and is logged. It reads the disk, so it never runs on
// the model goroutine.
func (s *Server) projectLayout(f tool.Folder) *layout.Template {
	if s.projects == nil {
		return nil
	}
	key := projectKey(f)
	raw, err := s.projects.LoadLayout(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		s.log.Warn("project layout not used", "project", key, "err", err)
		return nil
	}
	var tmpl layout.Template
	if err := json.Unmarshal(raw, &tmpl); err != nil {
		s.log.Warn("project layout not used", "project", key, "err", err)
		return nil
	}
	if err := tmpl.Validate(); err != nil {
		s.log.Warn("project layout not used", "project", key, "err", err)
		return nil
	}
	if !slices.Contains(tmpl.Kinds(), api.PaneChat) {
		s.log.Warn("project layout not used", "project", key, "err", "it has no chat")
		return nil
	}
	return &tmpl
}

// rememberLayout saves the layout of the first window of sess that holds a
// chat, for the project of that chat's folder, unless it is the layout
// saved last. The saver writes it later, off the model goroutine.
func (st *state) rememberLayout(sess *session) {
	saver := st.srv.projectSaver
	if saver == nil {
		return
	}
	for _, w := range sess.windows {
		id, ok := w.paneOfKind(api.PaneChat)
		if !ok {
			continue
		}
		c := w.panes[id].chat
		tmpl := layout.TemplateOf(w.tree, func(id layout.PaneID) string { return w.panes[int(id)].kind() })
		b, err := json.Marshal(tmpl)
		if err != nil {
			st.srv.log.Warn("project layout", "err", err)
			return
		}
		distro, _ := st.srv.distroOf(c.profile)
		f := tool.Folder{Path: c.folder, Distro: distro}
		fid := distro + "\x00" + c.folder
		if st.remembered[fid] == string(b) {
			return
		}
		st.remembered[fid] = string(b)
		saver.Changed(fid, func() string { return projectKey(f) }, b)
		return
	}
}

// restoreProject applies tmpl, a project's saved layout, to w, the first
// window of a new session, whose chat keeps its pane. A terminal leaf
// starts a new shell with the chat's profile in its folder. A layout that
// does not fit is logged and the window stays as it is.
func (st *state) restoreProject(sess *session, w *window, tmpl layout.Template) {
	id, ok := w.paneOfKind(api.PaneChat)
	if !ok {
		return
	}
	c := w.panes[id].chat
	notice, err := st.applyTemplate(sess, w, tmpl, "project", c.profile, c.folder)
	if err != nil {
		st.srv.log.Warn("project layout not used", "session", sess.name, "err", err)
		return
	}
	w.focused = id
	if notice != "" {
		st.srv.log.Info("project layout", "session", sess.name, "note", notice)
	}
}

// projectFolder is the folder of a prestarted chat, for its project key.
func (s *Server) projectFolder(chat *restoredChat) tool.Folder {
	distro, _ := s.distroOf(chat.profile)
	return tool.Folder{Path: chat.folder, Distro: distro}
}
