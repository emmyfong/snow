package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAddExclude: the worktree folder is added once, after the lines the
// user already has, with a newline before it when the file lacks one.
func TestAddExclude(t *testing.T) {
	tests := []struct {
		name     string
		existing *string // nil for no file
		want     string
	}{
		{"no file", nil, ".worktrees/\n"},
		{"empty file", ptr(""), ".worktrees/\n"},
		{"lines kept", ptr("# mine\n*.log\n"), "# mine\n*.log\n.worktrees/\n"},
		{"no newline at the end", ptr("*.log"), "*.log\n.worktrees/\n"},
		{"already there", ptr("*.log\n.worktrees/\n"), "*.log\n.worktrees/\n"},
		{"already there without a slash", ptr(".worktrees\n"), ".worktrees\n"},
		{"already there from the root", ptr("/.worktrees/\n"), "/.worktrees/\n"},
		{"already there with spaces around", ptr("  .worktrees/  \n"), "  .worktrees/  \n"},
		{"similar name is not it", ptr(".worktrees-old/\n"), ".worktrees-old/\n.worktrees/\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "info", "exclude")
			if tt.existing != nil {
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}
				write(t, file, *tt.existing)
			}
			for range 2 { // a second call changes nothing
				if err := addExclude(file); err != nil {
					t.Fatal(err)
				}
			}
			if got := string(readFile(t, file)); got != tt.want {
				t.Fatalf("exclude file %q, want %q", got, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }
