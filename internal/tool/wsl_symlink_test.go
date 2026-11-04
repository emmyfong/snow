package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
)

// wslTree makes a folder in a WSL distro with a file, links to that file and
// its folder, and links that lead outside the folder. Its tests run only on
// Windows with WSL.
func wslTree(t *testing.T) Env {
	t.Helper()
	wsl := wslProfile(t)
	dir := fmt.Sprintf("/tmp/snow-wsl-link-%d", os.Getpid())
	base := Env{Folder: Folder{Path: "/tmp", Distro: wsl.Distro}, Shell: wsl}
	setup := `rm -rf ` + dir +
		` && mkdir -p ` + dir + `/real` +
		` && printf 'one\ntwo\n' > ` + dir + `/real/a.txt` +
		` && ln -s real/a.txt ` + dir + `/link.txt` +
		` && ln -s real ` + dir + `/linkdir` +
		` && ln -s /etc/hostname ` + dir + `/outside.txt` +
		` && ln -s /no/such/snow-path ` + dir + `/dangling`
	if got := wslRun(t, base, setup); got.IsError {
		t.Fatalf("make the WSL tree: %+v", got)
	}
	t.Cleanup(func() { wslRun(t, base, `rm -rf `+dir) })
	return Env{Folder: Folder{Path: dir, Distro: wsl.Distro}, Shell: wsl}
}

// wslRun runs one command line through the run tool.
func wslRun(t *testing.T, env Env, line string) Result {
	t.Helper()
	in, err := json.Marshal(map[string]string{"command": line})
	if err != nil {
		t.Fatal(err)
	}
	return run(t, env, "run", string(in))
}

// TestWSLSymlinkTools: every file tool follows a link inside a WSL folder.
func TestWSLSymlinkTools(t *testing.T) {
	env := wslTree(t)
	tests := []struct{ name, tool, in, want string }{
		{"read", "read", `{"path":"link.txt"}`, "     1\tone\n     2\ttwo\n"},
		{"list a linked folder", "list", `{"path":"linkdir"}`, "a.txt\n"},
		{"search a linked file", "search", `{"pattern":"two","path":"link.txt"}`, "link.txt:2: two\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, env, tt.tool, tt.in); got.IsError || got.Content != tt.want {
				t.Fatalf("got %+v\nwant %q", got, tt.want)
			}
		})
	}
	if got := run(t, env, "edit", `{"path":"link.txt","old":"two","new":"2"}`); got.IsError {
		t.Fatalf("edit: %+v", got)
	}
	if got := run(t, env, "read", `{"path":"real/a.txt"}`); got.IsError || got.Content != "     1\tone\n     2\t2\n" {
		t.Fatalf("edit wrote elsewhere: %+v", got)
	}
	if got := run(t, env, "write", `{"path":"link.txt","content":"three\n"}`); got.IsError {
		t.Fatalf("write: %+v", got)
	}
	if got := run(t, env, "read", `{"path":"real/a.txt"}`); got.IsError || got.Content != "     1\tthree\n" {
		t.Fatalf("write wrote elsewhere: %+v", got)
	}
}

// TestWSLSymlinkOutsideAsks: a link that leads outside a WSL folder asks for
// approval, also when its target does not exist.
func TestWSLSymlinkOutsideAsks(t *testing.T) {
	env := wslTree(t)
	read, _ := Builtin().Get("read")
	for _, path := range []string{"outside.txt", "dangling"} {
		p, err := env.Folder.Resolve(path)
		if err != nil {
			t.Fatal(err)
		}
		if !env.Folder.Contains(p) {
			t.Fatalf("%s is not lexically inside; the test does not prove the distro check", path)
		}
		got := env.Folder.Outside(context.Background(), read, json.RawMessage(`{"path":"`+path+`"}`))
		if !slices.Equal(got, []string{path}) {
			t.Errorf("Outside(%s) = %v, want it outside", path, got)
		}
	}
	for _, path := range []string{"link.txt", "linkdir/a.txt"} {
		if got := env.Folder.Outside(context.Background(), read, json.RawMessage(`{"path":"`+path+`"}`)); got != nil {
			t.Errorf("an inside path %s asks: %v", path, got)
		}
	}
}
