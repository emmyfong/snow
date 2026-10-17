package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSetTheme(t *testing.T) {
	tests := []struct {
		name, before, after string
	}{
		{"no file", "", "[theme]\nname = \"Nord\"\n"},
		{"replaces the name", "[theme]\nname = \"default\"\n", "[theme]\nname = \"Nord\"\n"},
		{"keeps other lines and comments", "# my settings\n[keys]\nprefix = \"C-a\"\n\n[theme]\n# picked by hand\nname = 'default' # old\n\n[scrollback]\nlines = 5000\n",
			"# my settings\n[keys]\nprefix = \"C-a\"\n\n[theme]\n# picked by hand\nname = \"Nord\" # old\n\n[scrollback]\nlines = 5000\n"},
		{"table without a name", "[theme]\n\n[keys]\nprefix = \"C-a\"\n", "[theme]\nname = \"Nord\"\n\n[keys]\nprefix = \"C-a\"\n"},
		{"no theme table", "[keys]\nprefix = \"C-a\"\n", "[keys]\nprefix = \"C-a\"\n\n[theme]\nname = \"Nord\"\n"},
		{"dotted key", "theme.name = \"default\"\n", "theme.name = \"Nord\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "snow", "config.toml")
			if tt.before != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tt.before), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := SetTheme(path, "Nord"); err != nil {
				t.Fatalf("SetTheme: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.after {
				t.Fatalf("file:\n%s\nwant:\n%s", got, tt.after)
			}
			cfg, err := Load(path)
			if err != nil || cfg.Theme.Name != "Nord" {
				t.Fatalf("Load after SetTheme: %+v, %v", cfg.Theme, err)
			}
		})
	}
}

// TestSetThemeLeavesAnInlineTable: an inline table cannot change one key
// without rewriting the table; SetTheme refuses and leaves the file alone.
func TestSetThemeLeavesAnInlineTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "theme = { name = \"default\" }\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetTheme(path, "Nord"); !errors.Is(err, ErrCannotEdit) {
		t.Fatalf("SetTheme on an inline table: %v, want ErrCannotEdit", err)
	}
	if got, _ := os.ReadFile(path); string(got) != before {
		t.Fatalf("the file changed:\n%s", got)
	}
}
