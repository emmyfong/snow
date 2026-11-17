package files

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTree writes files under a new folder. A name ending in / is a folder.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestWalk: a walk visits every file and folder in lexical order, leaves out
// what git ignores, skips .git, and stops at fs.SkipDir.
func TestWalk(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		fn    func(rel string, d fs.DirEntry) error
		want  []string
	}{
		{
			name: "ignored files and folders",
			files: map[string]string{
				".gitignore":     "*.log\nbuild/\n",
				"a.txt":          "a",
				"skip.log":       "no",
				"build/out.txt":  "no",
				"dir/inner.txt":  "in",
				"dir/.gitignore": "*.tmp\n",
				"dir/skip.tmp":   "no",
				".git/HEAD":      "ref",
			},
			fn: nil,
			want: []string{
				".gitignore", "a.txt", "dir", "dir/.gitignore", "dir/inner.txt",
			},
		},
		{
			name: "skip dir",
			files: map[string]string{
				"a.txt":         "a",
				"sub/inner.txt": "in",
				"z.txt":         "z",
			},
			fn: func(rel string, d fs.DirEntry) error {
				if d.IsDir() && rel == "sub" {
					return fs.SkipDir
				}
				return nil
			},
			want: []string{"a.txt", "sub", "z.txt"},
		},
		{
			name: "skip all",
			files: map[string]string{
				"a.txt": "a",
				"b.txt": "b",
				"c.txt": "c",
			},
			fn: func(rel string, _ fs.DirEntry) error {
				if rel == "b.txt" {
					return fs.SkipAll
				}
				return nil
			},
			want: []string{"a.txt", "b.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeTree(t, tt.files)
			var got []string
			err := Walk(context.Background(), root, func(rel string, d fs.DirEntry) error {
				got = append(got, rel)
				if tt.fn == nil {
					return nil
				}
				return tt.fn(rel, d)
			})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("walk %q, want %q", got, tt.want)
			}
		})
	}
}

// TestListFilesCaps: ListFiles lists files only, stops at the limit, and
// says that the walk was cut.
func TestListFilesCaps(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.txt":         "a",
		"b.txt":         "b",
		"sub/c.txt":     "c",
		"sub/d.txt":     "d",
		"sub/deep/e.go": "e",
	})
	tests := []struct {
		name  string
		limit int
		want  []string
		cut   bool
	}{
		{"all", 10, []string{"a.txt", "b.txt", "sub/c.txt", "sub/d.txt", "sub/deep/e.go"}, false},
		{"exact", 5, []string{"a.txt", "b.txt", "sub/c.txt", "sub/d.txt", "sub/deep/e.go"}, false},
		{"over", 4, []string{"a.txt", "b.txt", "sub/c.txt", "sub/d.txt"}, true},
		{"two", 2, []string{"a.txt", "b.txt"}, true},
		{"none", 0, nil, false},
		{"negative", -1, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cut, err := ListFiles(context.Background(), root, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) || cut != tt.cut {
				t.Fatalf("ListFiles(%d) = %q, cut %v; want %q, cut %v", tt.limit, got, cut, tt.want, tt.cut)
			}
		})
	}
}
