package convstore

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// listWorkers is how many files ListAll reads at once.
const listWorkers = 8

// maxTitle is the length, in characters, of a title made from the first
// user message.
const maxTitle = 60

// Entry describes one conversation for a history list.
type Entry struct {
	ID      string
	Path    string
	Title   string
	Folder  string
	Profile string
	Model   string // "provider/model": the latest switch, else the meta model
	// LastActivity is the time of the latest record; for a file that cannot
	// be read, its modification time.
	LastActivity time.Time
	Turns        int   // user messages
	Err          error // a *LoadError when the file cannot be read; the file is kept
}

// List returns the conversations of one folder and shell profile, newest
// first. Folder and profile must both match, byte for byte: a WSL folder
// and its Windows path are different folders. The caller cleans the folder
// the same way when it writes Meta.Folder and when it lists, so that
// C:\Proj and c:\proj\ do not give two histories. A file that cannot be read is in the list
// with its error, whatever its folder, so the user can see it.
func (s *Store) List(folder, profile string) ([]Entry, error) {
	all, err := s.ListAll()
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(e Entry) bool {
		return e.Err == nil && (e.Folder != folder || e.Profile != profile)
	}), nil
}

// ListAll returns every conversation, newest first. Only a failure to read
// the folder itself gives an error.
func (s *Store) ListAll() ([]Entry, error) {
	files, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	var (
		ids  []string
		dirs []os.DirEntry
	)
	for _, f := range files {
		id, ok := strings.CutSuffix(f.Name(), ext)
		if !ok || f.IsDir() {
			continue
		}
		// Only a crash inside Create leaves an empty file. It holds no
		// conversation, so it is not shown.
		if info, err := f.Info(); err == nil && info.Size() == 0 {
			continue
		}
		ids, dirs = append(ids, id), append(dirs, f)
	}
	entries := make([]Entry, len(ids))
	// Files are read in parallel: on a slow disk, waiting dominates.
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(listWorkers, len(ids)) {
		wg.Go(func() {
			for i := range next {
				entries[i] = s.entry(ids[i], dirs[i])
			}
		})
	}
	for i := range ids {
		next <- i
	}
	close(next)
	wg.Wait()
	slices.SortStableFunc(entries, func(a, b Entry) int {
		return cmp.Or(b.LastActivity.Compare(a.LastActivity), strings.Compare(a.ID, b.ID))
	})
	return entries, nil
}

// partialPrefix starts every line that Encode writes for a Partial.
var partialPrefix = []byte(`{"type":"partial"`)

// head is the part of a record that the list needs from every line.
type head struct {
	Type Type      `json:"type"`
	At   time.Time `json:"at"`
}

// entry reads one file for the list. It decodes in full only the records
// that the list shows, which are small: big replies and tool output are
// read for their type and time. A torn or damaged line does not make the
// file an error; only what stops Load does.
func (s *Store) entry(id string, f os.DirEntry) Entry {
	path := filepath.Join(s.dir, f.Name())
	e := Entry{ID: id, Path: path}
	if err := CheckID(id); err != nil {
		// A file the user renamed or copied in: show it, never skip it.
		return failed(e, f, err)
	}
	var (
		meta    *Meta
		first   = true
		renamed bool
	)
	err := eachLine(path, func(_ int, b []byte, _ bool) {
		if first {
			first = false
			if r, err := Decode(b); err == nil {
				meta, _ = r.(*Meta)
			}
			return
		}
		// Each partial holds the whole reply so far, so partial lines are
		// the biggest; the reply or the next record dates the turn.
		if bytes.HasPrefix(b, partialPrefix) {
			return
		}
		var h head
		if json.Unmarshal(b, &h) != nil {
			return
		}
		if h.At.After(e.LastActivity) {
			e.LastActivity = h.At
		}
		switch h.Type {
		case TypeUser, TypeRename, TypeModelSwitch:
		default:
			return
		}
		r, err := Decode(b)
		if err != nil {
			return
		}
		switch r := r.(type) {
		case *User:
			e.Turns++
			if !renamed && e.Title == "" {
				e.Title = titleFrom(r.Text)
			}
		case *Rename:
			e.Title, renamed = r.Title, true
		case *ModelSwitch:
			e.Model = r.Model
		}
	})
	if err == nil {
		err = checkMeta(meta, id)
	}
	if err != nil {
		return failed(Entry{ID: id, Path: path}, f, err)
	}
	if !renamed && meta.Title != "" {
		e.Title = meta.Title
	}
	if e.Model == "" {
		e.Model = meta.Model
	}
	if meta.Created.After(e.LastActivity) {
		e.LastActivity = meta.Created
	}
	e.Folder, e.Profile = meta.Folder, meta.Profile
	return e
}

// failed returns e as the entry of a file that cannot be read, dated by
// the file's modification time.
func failed(e Entry, f os.DirEntry, err error) Entry {
	e.Err = &LoadError{Path: e.Path, Err: err}
	if info, err := f.Info(); err == nil {
		e.LastActivity = info.ModTime()
	}
	return e
}

// titleFrom makes a one-line title of at most maxTitle characters from a
// message.
func titleFrom(text string) string {
	title := OneLine(text)
	if r := []rune(title); len(r) > maxTitle {
		title = strings.TrimRightFunc(string(r[:maxTitle-1]), unicode.IsSpace) + "…"
	}
	return title
}
