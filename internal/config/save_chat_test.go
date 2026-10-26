package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSetChatModel(t *testing.T) {
	tests := []struct {
		name, before, after string
	}{
		{"no file", "", "[chat]\neffort = \"high\"\nmodel = \"anthropic/claude-opus-5-5\"\n"},
		{"replaces both", "[chat]\nmodel = 'openai/gpt-6.1-sol'\neffort = 'low'\n",
			"[chat]\nmodel = \"anthropic/claude-opus-5-5\"\neffort = \"high\"\n"},
		{"keeps other lines and comments", "# my settings\n[chat]\n# chosen by hand\nmodel = 'openai/gpt-6.1-sol' # old\nusage = true\n\n[theme]\nname = \"Nord\"\n",
			"# my settings\n[chat]\neffort = \"high\"\n# chosen by hand\nmodel = \"anthropic/claude-opus-5-5\" # old\nusage = true\n\n[theme]\nname = \"Nord\"\n"},
		{"table without a model", "[chat]\nusage = true\n", "[chat]\neffort = \"high\"\nmodel = \"anthropic/claude-opus-5-5\"\nusage = true\n"},
		{"no chat table", "[theme]\nname = \"Nord\"\n", "[theme]\nname = \"Nord\"\n\n[chat]\neffort = \"high\"\nmodel = \"anthropic/claude-opus-5-5\"\n"},
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
			if err := SetChatModel(path, "anthropic/claude-opus-5-5", "high"); err != nil {
				t.Fatalf("SetChatModel: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.after {
				t.Fatalf("file:\n%s\nwant:\n%s", got, tt.after)
			}
			cfg, err := Load(path)
			if err != nil || cfg.Chat.Model != "anthropic/claude-opus-5-5" || cfg.Chat.Effort != "high" {
				t.Fatalf("Load after SetChatModel: %+v, %v", cfg.Chat, err)
			}
		})
	}
}

// TestSetChatModelEmptyEffort writes the empty effort, which lets the
// provider choose.
func TestSetChatModelEmptyEffort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := SetChatModel(path, "openai/gpt-6.1-sol", ""); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.Chat.Model != "openai/gpt-6.1-sol" || cfg.Chat.Effort != "" {
		t.Fatalf("Load after SetChatModel: %+v, %v", cfg.Chat, err)
	}
}

func TestSetChatModelRejectsANameWithoutAProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := SetChatModel(path, "opus", ""); err == nil {
		t.Fatal("SetChatModel accepted a model without a provider")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the file was created: %v", err)
	}
}

// TestSetChatModelLeavesAnInlineTable: an inline table cannot change one
// key without rewriting the table; SetChatModel refuses and leaves the file
// alone.
func TestSetChatModelLeavesAnInlineTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "chat = { model = \"openai/gpt-6.1-sol\" }\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetChatModel(path, "anthropic/claude-opus-5-5", "high"); !errors.Is(err, ErrCannotEdit) {
		t.Fatalf("SetChatModel on an inline table: %v, want ErrCannotEdit", err)
	}
	if got, _ := os.ReadFile(path); string(got) != before {
		t.Fatalf("the file changed:\n%s", got)
	}
}

// TestSetChatModelLeavesADottedKey: a dotted key makes the table, and TOML
// does not let a header define it again. The writer refuses instead of
// writing a file that would not load.
func TestSetChatModelLeavesADottedKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "chat.model = \"openai/gpt-6.1-sol\"\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetChatModel(path, "anthropic/claude-opus-5-5", "high"); !errors.Is(err, ErrCannotEdit) {
		t.Fatalf("SetChatModel on a dotted key: %v, want ErrCannotEdit", err)
	}
	if got, _ := os.ReadFile(path); string(got) != before {
		t.Fatalf("the file changed:\n%s", got)
	}
}
