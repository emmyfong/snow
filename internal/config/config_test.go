package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeConfig writes body to a config file in a fresh temp folder and
// returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{
		filepath.Join(dir, "config.toml"),
		filepath.Join(dir, "no-such-folder", "config.toml"),
	} {
		got, err := Load(path)
		if err != nil {
			t.Fatalf("Load(%q) error = %v, want nil", path, err)
		}
		if !reflect.DeepEqual(got, Defaults()) {
			t.Errorf("Load(%q) = %+v, want Defaults() %+v", path, got, Defaults())
		}
	}
}

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.Keys.Prefix != "C-b" {
		t.Errorf("Keys.Prefix = %q, want %q", d.Keys.Prefix, "C-b")
	}
	if d.Scrollback.Lines != 2000 {
		t.Errorf("Scrollback.Lines = %d, want 2000", d.Scrollback.Lines)
	}
	if d.Theme.Name != "default" {
		t.Errorf("Theme.Name = %q, want %q", d.Theme.Name, "default")
	}
	if err := d.validate(); err != nil {
		t.Errorf("Defaults().validate() = %v, want nil", err)
	}
}

func TestOverrides(t *testing.T) {
	tests := []struct {
		name string
		body string
		want func(*Config)
	}{
		{
			name: "prefix and scrollback",
			body: "[keys]\nprefix = \"C-a\"\n\n[scrollback]\nlines = 5000\n",
			want: func(c *Config) {
				c.Keys.Prefix = "C-a"
				c.Scrollback.Lines = 5000
			},
		},
		{
			name: "empty file keeps defaults",
			body: "",
			want: func(*Config) {},
		},
		{
			name: "comments only keep defaults",
			body: "# nothing here\n",
			want: func(*Config) {},
		},
		{
			name: "every field",
			body: `[keys]
prefix = "C-a"

[keys.bind]
"|" = "split-right"
"-" = "split-down"
"%" = "none"

[scrollback]
lines = 10000

[profile]
default = "Ubuntu"

[theme]
name = "light"
`,
			want: func(c *Config) {
				c.Keys.Prefix = "C-a"
				c.Keys.Bind = map[string]string{"|": "split-right", "-": "split-down", "%": "none"}
				c.Scrollback.Lines = 10000
				c.Profile.Default = "Ubuntu"
				c.Theme.Name = "light"
			},
		},
		{
			name: "several keys for one action",
			body: "[keys.bind]\n\"|\" = \"split-right\"\n\".\" = \"split-right\"\n\"C-Left\" = \"focus-left\"\n",
			want: func(c *Config) {
				c.Keys.Bind = map[string]string{"|": "split-right", ".": "split-right", "C-Left": "focus-left"}
			},
		},
		{
			name: "dotted keys and inline table",
			body: "keys.prefix = \"C-a\"\ntheme = { name = \"light\" }\n",
			want: func(c *Config) {
				c.Keys.Prefix = "C-a"
				c.Theme.Name = "light"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(writeConfig(t, tt.body))
			if err != nil {
				t.Fatalf("Load error = %v, want nil", err)
			}
			want := Defaults()
			tt.want(&want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Load = %+v, want %+v", got, want)
			}
		})
	}
}

func TestInvalidKeyReportsLine(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantLine int
		wantKey  string
		wantText []string // substrings the message must hold
	}{
		{
			name:     "syntax error",
			body:     "[keys]\nprefix = \"C-a\"\nbroken =\n",
			wantLine: 3,
			wantText: []string{"line 3"},
		},
		{
			name:     "unknown key",
			body:     "[keys]\nprefix = \"C-a\"\nprefx = \"C-b\"\n",
			wantLine: 3,
			wantKey:  "keys.prefx",
			wantText: []string{"line 3", `"keys.prefx"`, "unknown key"},
		},
		{
			name:     "unknown table",
			body:     "[scrollback]\nlines = 10\n\n[colour]\nname = \"x\"\n",
			wantLine: 4,
			wantKey:  "colour",
			wantText: []string{"line 4", `"colour"`, "unknown key"},
		},
		{
			name:     "unknown top-level key",
			body:     "# my config\nfont = \"mono\"\n",
			wantLine: 2,
			wantKey:  "font",
			wantText: []string{"line 2", `"font"`},
		},
		{
			name:     "unknown key in inline table points at the table",
			body:     "\nkeys = { prefix = \"C-a\", bogus = 1 }\n",
			wantLine: 2,
			wantKey:  "keys.bogus",
			wantText: []string{"line 2", `"keys.bogus"`},
		},
		{
			name:     "wrong type",
			body:     "[scrollback]\n\nlines = \"many\"\n",
			wantText: []string{"line 3", "scrollback.lines"},
		},
		{
			name:     "negative scrollback",
			body:     "[scrollback]\nlines = -5\n",
			wantLine: 2,
			wantKey:  "scrollback.lines",
			wantText: []string{"line 2", `"scrollback.lines"`, "at least 1"},
		},
		{
			name:     "zero scrollback",
			body:     "[scrollback]\nlines = 0\n",
			wantLine: 2,
			wantKey:  "scrollback.lines",
		},
		{
			name:     "empty prefix",
			body:     "[theme]\nname = \"light\"\n[keys]\nprefix = \"\"\n",
			wantLine: 4,
			wantKey:  "keys.prefix",
			wantText: []string{"line 4", `"keys.prefix"`},
		},
		{
			name:     "empty action",
			body:     "[keys.bind]\n\"|\" = \"split-right\"\n\".\" = \"\"\n",
			wantLine: 3,
			wantKey:  "keys.bind.\".\"",
			wantText: []string{"line 3", "use \"none\" to unbind"},
		},
		{
			name:     "empty key",
			body:     "[keys.bind]\n\"\" = \"split-right\"\n",
			wantLine: 2,
			wantText: []string{"line 2", "the key must not be empty"},
		},
		{
			name:     "bind value of the wrong type",
			body:     "[keys.bind]\n\"|\" = 5\n",
			wantText: []string{"line 2"},
		},
		{
			name:     "empty theme",
			body:     "[theme]\nname = \"\"\n",
			wantLine: 2,
			wantKey:  "theme.name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.body)
			got, err := Load(path)
			if err == nil {
				t.Fatalf("Load error = nil, want an error; got %+v", got)
			}
			var ce *Error
			if !errors.As(err, &ce) {
				t.Fatalf("Load error %v (%T) is not a *Error", err, err)
			}
			if ce.Path != path {
				t.Errorf("Path = %q, want %q", ce.Path, path)
			}
			if tt.wantLine != 0 && ce.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d (%v)", ce.Line, tt.wantLine, err)
			}
			if tt.wantKey != "" && ce.Key != tt.wantKey {
				t.Errorf("Key = %q, want %q (%v)", ce.Key, tt.wantKey, err)
			}
			msg := err.Error()
			for _, s := range append(tt.wantText, path) {
				if !strings.Contains(msg, s) {
					t.Errorf("message %q does not contain %q", msg, s)
				}
			}
		})
	}
}

func TestUnreadableFile(t *testing.T) {
	// A folder where the file should be cannot be read as a file.
	path := t.TempDir()
	_, err := Load(path)
	var ce *Error
	if !errors.As(err, &ce) {
		t.Fatalf("Load(folder) error = %v, want a *Error", err)
	}
	if ce.Err == nil {
		t.Errorf("Err = nil, want the read error")
	}
}

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this on Windows
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".snow", "config.toml")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestDefaultsAreIndependent(t *testing.T) {
	a := Defaults()
	a.Keys.Bind = map[string]string{"x": "y"}
	if b := Defaults(); b.Keys.Bind != nil {
		t.Errorf("Defaults() shares state between calls: Bind = %v", b.Keys.Bind)
	}
}
