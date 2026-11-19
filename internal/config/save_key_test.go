package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSetKeyKeepsComments(t *testing.T) {
	tests := []struct {
		name, before, want string
		key                []string
	}{
		{name: "no file", key: []string{"theme", "name"}, want: "[theme]\nname = \"Nord\"\n"},
		{name: "missing table", before: "# keep\n[keys]\nprefix = \"C-a\"\n", key: []string{"keys", "bind", "x"}, want: "# keep\n[keys]\nprefix = \"C-a\"\n\n[keys.bind]\nx = \"Nord\"\n"},
		{name: "missing key", before: "[theme]\n# keep\nbackground = \"terminal\"\n", key: []string{"theme", "name"}, want: "[theme]\nname = \"Nord\"\n# keep\nbackground = \"terminal\"\n"},
		{name: "trailing comment", before: "[theme]\nname = 'old' # keep\n", key: []string{"theme", "name"}, want: "[theme]\nname = \"Nord\" # keep\n"},
		{name: "quoted key", before: "[keys.bind]\n# keep\n", key: []string{"keys", "bind", "|"}, want: "[keys.bind]\n\"|\" = \"Nord\"\n# keep\n"},
		{name: "equals in quoted key", before: "[keys.bind]\n\"=\" = \"split-right\"\n", key: []string{"keys", "bind", "="}, want: "[keys.bind]\n\"=\" = \"Nord\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if tt.before != "" {
				if err := os.WriteFile(path, []byte(tt.before), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := SetKey(path, tt.key, Quote("Nord")); err != nil {
				t.Fatalf("SetKey: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("file:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestSetKeyLeavesInlineTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "keys = { bind = { x = \"old\" } }\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetKey(path, []string{"keys", "bind", "x"}, Quote("new")); !errors.Is(err, ErrCannotEdit) {
		t.Fatalf("SetKey: %v, want ErrCannotEdit", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != before {
		t.Fatalf("file changed: %q, %v", got, err)
	}
}

func TestSetKeyRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	values := map[string]string{"|": "split-right", "x": "close-pane"}
	for key, value := range values {
		if err := SetKey(path, []string{"keys", "bind", key}, Quote(value)); err != nil {
			t.Fatalf("SetKey(%q): %v", key, err)
		}
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range values {
		if got := cfg.Keys.Bind[key]; got != want {
			t.Errorf("Keys.Bind[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestSetKeyReplacesScalarLiterals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "[scrollback]\nlines = 2000 # keep lines\n[chat]\nusage = true # keep usage\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetKey(path, []string{"scrollback", "lines"}, "3000"); err != nil {
		t.Fatalf("SetKey integer: %v", err)
	}
	if err := SetKey(path, []string{"chat", "usage"}, "false"); err != nil {
		t.Fatalf("SetKey boolean: %v", err)
	}
	want := "[scrollback]\nlines = 3000 # keep lines\n[chat]\nusage = false # keep usage\n"
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("file:\n%s\nwant:\n%s\nerror: %v", got, want, err)
	}
}
