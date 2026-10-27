package claudecode

import (
	"errors"
	"strings"
	"testing"
)

const snowCmd = `'/usr/local/bin/snow' hook`

// otherHooks is a settings file with the user's own settings and hooks, in
// Claude Code's own format: two-space indent, a newline at the end.
const otherHooks = `{
  "model": "opus",
  "permissions": {
    "allow": [
      "Bash(go test:*)"
    ]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "~/bin/guard.sh"
          }
        ]
      }
    ]
  },
  "env": {
    "A": "\u00e9 <b>"
  }
}
`

func TestClaudeSetup(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr error
		reshape bool // a compact file comes back indented
	}{
		{name: "no file", in: ""},
		{name: "an empty file", in: "  \n"},
		{name: "an empty object", in: "{}\n"},
		{name: "a file with other hooks", in: otherHooks},
		{name: "four spaces", in: strings.ReplaceAll(otherHooks, "  ", "    ")},
		{name: "tabs", in: strings.ReplaceAll(otherHooks, "  ", "\t")},
		{name: "CRLF line endings", in: strings.ReplaceAll(otherHooks, "\n", "\r\n")},
		{name: "a byte order mark", in: "\uFEFF" + otherHooks},
		{name: "no newline at the end", in: strings.TrimSuffix(otherHooks, "\n")},
		{name: "compact", in: `{"model":"opus","hooks":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}`, reshape: true},
		{name: "a file Snow cannot parse", in: `{"model": "opus",`, wantErr: ErrUnreadable},
		{name: "not an object", in: `["hooks"]`, wantErr: ErrUnreadable},
		{name: "hooks of another shape", in: `{"hooks": []}`, wantErr: ErrUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, changed, err := AddHooks([]byte(tt.in), snowCmd)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || out != nil {
					t.Fatalf("AddHooks() = %q, %v; want no output and %v", out, err, tt.wantErr)
				}
				return
			}
			if err != nil || !changed {
				t.Fatalf("AddHooks() changed %v, %v; want a change", changed, err)
			}
			st, err := HookStatus(out)
			if err != nil || !st.Set() {
				t.Fatalf("status after setup %+v, %v; want every event set\n%s", st, err, out)
			}
			// Running setup twice changes nothing the second time.
			again, changed, err := AddHooks(out, snowCmd)
			if err != nil || changed || string(again) != string(out) {
				t.Fatalf("second setup changed the file:\n%s", again)
			}
			// Remove gives back the user's file, byte for byte.
			back, _, err := RemoveHooks(out)
			if err != nil {
				t.Fatal(err)
			}
			want := tt.in
			if strings.TrimSpace(want) == "" {
				want = "{}\n"
			}
			if tt.reshape {
				if st, err := HookStatus(back); err != nil || !st.None() || !strings.Contains(string(back), `"command": "x"`) {
					t.Fatalf("setup then remove of a compact file =\n%s", back)
				}
				return
			}
			if string(back) != want {
				t.Fatalf("setup then remove =\n%s\nwant\n%s", back, want)
			}
		})
	}
}

// TestClaudeSetupKeepsOtherSettings: the user's settings and hooks stay
// as they were, in their order, escapes included.
func TestClaudeSetupKeepsOtherSettings(t *testing.T) {
	out, _, err := AddHooks([]byte(otherHooks), snowCmd)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"{\n  \"model\": \"opus\",\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(go test:*)\"\n    ]\n  },\n  \"hooks\": {\n    \"PreToolUse\": [\n      {\n        \"matcher\": \"Bash\",",
		`"command": "~/bin/guard.sh"`,
		`"A": "\u00e9 <b>"`,
		`"command": "'/usr/local/bin/snow' hook"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("output lacks %q:\n%s", want, s)
		}
	}
	if strings.Index(s, `"env"`) < strings.Index(s, `"hooks"`) {
		t.Fatalf("keys were reordered:\n%s", s)
	}
}

// TestClaudeSetupMovedSnow: a Snow hook with another path is replaced, not
// added twice.
func TestClaudeSetupMovedSnow(t *testing.T) {
	old, _, err := AddHooks(nil, `'/opt/snow/snow' hook`)
	if err != nil {
		t.Fatal(err)
	}
	out, changed, err := AddHooks(old, snowCmd)
	if err != nil || !changed {
		t.Fatalf("AddHooks() = %v, %v", changed, err)
	}
	if strings.Contains(string(out), "/opt/snow") || strings.Count(string(out), snowCmd) != len(HookEvents) {
		t.Fatalf("old path kept or new one missing:\n%s", out)
	}
}

func TestClaudeRemove(t *testing.T) {
	both, _, err := AddHooks([]byte(otherHooks), snowCmd)
	if err != nil {
		t.Fatal(err)
	}
	// The user put their own command in Snow's group too.
	mixed := strings.Replace(string(both), `"command": "'/usr/local/bin/snow' hook"`, `"command": "'/usr/local/bin/snow' hook"
          },
          {
            "type": "command",
            "command": "notify-send stop"`, 1)
	out, changed, err := RemoveHooks([]byte(mixed))
	if err != nil || !changed {
		t.Fatalf("RemoveHooks() = %v, %v", changed, err)
	}
	s := string(out)
	if strings.Contains(s, "snow' hook") {
		t.Fatalf("a Snow hook is left:\n%s", s)
	}
	for _, want := range []string{"~/bin/guard.sh", "notify-send stop", `"model": "opus"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("remove took %q too:\n%s", want, s)
		}
	}
	if _, changed, _ := RemoveHooks(out); changed {
		t.Fatal("a second remove changed the file")
	}
	if _, _, err := RemoveHooks([]byte(`{"hooks": `)); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("RemoveHooks(broken) = %v, want ErrUnreadable", err)
	}
}

func TestIsSnowCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{`'/usr/local/bin/snow' hook`, true},
		{`"C:/Users/emmy/AppData/Local/snow/snow.exe" hook`, true},
		{`'/mnt/c/Users/emmy/AppData/Local/snow/snow.exe' hook`, true},
		{`snow hook`, true},
		{`/usr/bin/snowball hook`, false},
		{`'/usr/local/bin/snow' version`, false},
		{`~/bin/guard.sh`, false},
		{`echo snow hook`, false},
	}
	for _, tt := range tests {
		if got := IsSnowCommand(tt.cmd); got != tt.want {
			t.Errorf("IsSnowCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

func TestHookCommand(t *testing.T) {
	tests := []struct{ exe, want string }{
		{"/usr/local/bin/snow", `'/usr/local/bin/snow' hook`},
		{`C:\Users\emmy\AppData\Local\snow\snow.exe`, `'C:/Users/emmy/AppData/Local/snow/snow.exe' hook`},
		{"/home/o'neil/bin/snow", `'/home/o'\''neil/bin/snow' hook`},
	}
	for _, tt := range tests {
		if got := HookCommand(tt.exe); got != tt.want {
			t.Errorf("HookCommand(%q) = %q, want %q", tt.exe, got, tt.want)
		}
		if !IsSnowCommand(HookCommand(tt.exe)) {
			t.Errorf("IsSnowCommand does not know HookCommand(%q)", tt.exe)
		}
	}
}

func TestHookStatus(t *testing.T) {
	st, err := HookStatus([]byte(otherHooks))
	if err != nil || st.Set() || len(st.Missing) != len(HookEvents) {
		t.Fatalf("status of a file without Snow = %+v, %v", st, err)
	}
	out, _, _ := AddHooks(nil, snowCmd)
	partial := strings.Replace(string(out), `"Stop"`, `"Stopped"`, 1)
	st, err = HookStatus([]byte(partial))
	if err != nil || st.Set() || len(st.Missing) != 1 || st.Missing[0] != EventStop {
		t.Fatalf("status with Stop missing = %+v, %v", st, err)
	}
}

// TestCheckProgram: setup refuses a snow that runs under another name; its
// hook could not be told from other commands.
func TestCheckProgram(t *testing.T) {
	tests := []struct {
		exe string
		ok  bool
	}{
		{"/usr/local/bin/snow", true},
		{`C:\Users\emmy\AppData\Local\snow\snow.exe`, true},
		{`C:\bin\SNOW.EXE`, true},
		{"/home/u/Downloads/snow-linux-amd64", false},
		{"/home/u/go/bin/snow-dev", false},
	}
	for _, tt := range tests {
		err := CheckProgram(tt.exe)
		if (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrNotSnow)) {
			t.Errorf("CheckProgram(%q) = %v, want ok %v", tt.exe, err, tt.ok)
		}
	}
}
