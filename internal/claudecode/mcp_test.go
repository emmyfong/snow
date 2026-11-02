package claudecode

import (
	"bufio"
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// recorder stands in for the claude program: it records every call, answers
// `mcp get` with the server's scope, and fails `mcp remove` when the user
// scope has none.
type recorder struct {
	// user is set while a server named snow is registered at user scope,
	// other while one is registered at another scope only.
	user, other bool
	calls       [][]string
	err         error
}

func (r *recorder) run(args []string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{}, args...))
	if r.err != nil {
		return nil, r.err
	}
	switch {
	case isGet(args):
		switch {
		case r.user:
			return []byte("snow:\n  Scope: User config (available in all your projects)\n"), nil
		case r.other:
			return []byte("snow:\n  Scope: Project config (shared via .mcp.json)\n"), nil
		}
		return nil, ErrNoServer
	case isRemove(args):
		if !r.user {
			return nil, ErrNoServer
		}
		r.user = false
		return nil, nil
	}
	return nil, nil
}

// isRemove reports whether args remove one MCP server.
func isRemove(args []string) bool {
	return len(args) >= 2 && args[0] == "mcp" && args[1] == "remove"
}

// mcp returns the registration that runs through r.
func (r *recorder) mcp() MCP {
	return MCP{Name: "Test Claude", Program: "claude", Exe: "/opt/snow", run: r.run}
}

// joined returns the recorded calls as one line each.
func (r *recorder) joined() []string {
	out := make([]string, len(r.calls))
	for i, call := range r.calls {
		out[i] = strings.Join(call, " ")
	}
	return out
}

func TestMCPStatus(t *testing.T) {
	notSet := &recorder{}
	if set, err := notSet.mcp().Status(); set || err != nil {
		t.Fatalf("Status without the server = %v, %v", set, err)
	}
	set := &recorder{user: true}
	if ok, err := set.mcp().Status(); !ok || err != nil {
		t.Fatalf("Status with the server = %v, %v", ok, err)
	}
	other := &recorder{other: true}
	if ok, err := other.mcp().Status(); ok || err != nil {
		t.Fatalf("Status with the server at another scope = %v, %v", ok, err)
	}
}

// TestMCPAdd: a first setup adds the server once; a setup over an existing
// one replaces it, so a moved snow program is updated.
func TestMCPAdd(t *testing.T) {
	fresh := &recorder{}
	if err := fresh.mcp().Add(); err != nil {
		t.Fatal(err)
	}
	want := []string{"mcp get snow", "mcp add --scope user snow -- /opt/snow mcp"}
	if got := fresh.joined(); !equalLines(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}

	again := &recorder{user: true}
	if err := again.mcp().Add(); err != nil {
		t.Fatal(err)
	}
	want = []string{"mcp get snow", "mcp remove --scope user snow", "mcp add --scope user snow -- /opt/snow mcp"}
	if got := again.joined(); !equalLines(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

// TestMCPAddOtherScope: a server of the same name in another scope is not
// Snow's, so setup adds Snow's own and does not fail on a remove it cannot
// make.
func TestMCPAddOtherScope(t *testing.T) {
	r := &recorder{other: true}
	if err := r.mcp().Add(); err != nil {
		t.Fatal(err)
	}
	want := []string{"mcp get snow", "mcp add --scope user snow -- /opt/snow mcp"}
	if got := r.joined(); !equalLines(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestMCPRemove(t *testing.T) {
	set := &recorder{user: true}
	if err := set.mcp().Remove(); err != nil {
		t.Fatal(err)
	}
	want := []string{"mcp remove --scope user snow"}
	if got := set.joined(); !equalLines(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
	none := &recorder{}
	if err := none.mcp().Remove(); err != nil {
		t.Fatal(err)
	}
	if got := none.joined(); !equalLines(got, want) {
		t.Fatalf("calls = %q, want one remove", got)
	}
}

func TestMCPWithoutClaude(t *testing.T) {
	m := MCP{}
	if set, err := m.Status(); set || err != nil {
		t.Fatalf("Status = %v, %v", set, err)
	}
	if err := m.Add(); err == nil {
		t.Fatal("Add without claude = nil")
	}
	if err := m.Remove(); err != nil {
		t.Fatalf("Remove without claude = %v", err)
	}
}

func TestMCPFailure(t *testing.T) {
	boom := errors.New("boom")
	r := &recorder{err: boom}
	if err := r.mcp().Add(); !errors.Is(err, boom) {
		t.Fatalf("Add = %v, want the claude error", err)
	}
}

// TestWSLMCP: the WSL registration runs wsl.exe and the user's login shell,
// with snow.exe's path as the distribution sees it.
func TestWSLMCP(t *testing.T) {
	m := WSLMCP("/usr/bin/wsl.exe", "Ubuntu", "/mnt/c/Program Files/snow.exe")
	if m.Name != "Claude Code in Ubuntu" || m.Program != "/usr/bin/wsl.exe" || m.Exe != "/mnt/c/Program Files/snow.exe" {
		t.Fatalf("m = %+v", m)
	}
	want := []string{"-d", "Ubuntu", "-e", "sh", "-c"}
	if len(m.Args) < len(want)+1 {
		t.Fatalf("args = %q", m.Args)
	}
	for i, w := range want {
		if m.Args[i] != w {
			t.Fatalf("args[%d] = %q, want %q", i, m.Args[i], w)
		}
	}
	if !strings.Contains(m.Args[5], `exec "$shell" -lic 'exec claude "$@"' sh "$@"`) {
		t.Fatalf("script = %q", m.Args[5])
	}
	if m.Command() != "'/mnt/c/Program Files/snow.exe' mcp" {
		t.Fatalf("Command = %q", m.Command())
	}
}

// TestRegisterMCPAsks: without a yes nothing changes; with one, the server
// is added, and a second setup says it is already set.
func TestRegisterMCPAsks(t *testing.T) {
	r := &recorder{}
	var out bytes.Buffer
	if err := registerMCP(r.mcp(), answer("n\n"), &out); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 || !strings.Contains(out.String(), "[y/N]") {
		t.Fatalf("calls = %q, output %q", r.joined(), out.String())
	}
	out.Reset()
	if err := registerMCP(r.mcp(), answer("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	if got := r.joined(); !strings.Contains(got[len(got)-1], "mcp add") {
		t.Fatalf("calls = %q", got)
	}
	out.Reset()
	r.user = true
	if err := registerMCP(r.mcp(), answer(""), &out); err != nil || !strings.Contains(out.String(), "already set") {
		t.Fatalf("second setup = %v, %q", err, out.String())
	}
}

func TestRegisterMCPWithoutClaude(t *testing.T) {
	var out bytes.Buffer
	if err := registerMCP(MCP{}, answer(""), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not on PATH") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestUnregisterMCP(t *testing.T) {
	r := &recorder{user: true}
	var out bytes.Buffer
	if err := unregisterMCP(r.mcp(), answer("n\n"), &out); err != nil {
		t.Fatal(err)
	}
	if got := r.joined(); len(got) != 1 {
		t.Fatalf("a no removed something: %q", got)
	}
	out.Reset()
	if err := unregisterMCP(r.mcp(), answer("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	if got := r.joined(); !strings.Contains(got[len(got)-1], "mcp remove") {
		t.Fatalf("calls = %q", got)
	}
}

func TestReportMCP(t *testing.T) {
	var out bytes.Buffer
	if err := reportMCP((&recorder{}).mcp(), &out); err != nil || !strings.Contains(out.String(), "not set") {
		t.Fatalf("report = %q, %v", out.String(), err)
	}
	out.Reset()
	if err := reportMCP((&recorder{user: true}).mcp(), &out); err != nil || !strings.Contains(out.String(), "is set") {
		t.Fatalf("report = %q, %v", out.String(), err)
	}
}

// TestSetupAsksForBoth: setup asks for the hooks and then for the MCP
// server, and writes both.
func TestSetupAsksForBoth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	r := &recorder{}
	var out bytes.Buffer
	in := bufio.NewReader(strings.NewReader("y\ny\n"))
	if err := Setup(FileTarget{Path: path, Cmd: snowCmd}, r.mcp(), in, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "[y/N]") != 2 {
		t.Fatalf("setup asked %d times:\n%s", strings.Count(out.String(), "[y/N]"), out.String())
	}
	if got := r.joined(); !strings.Contains(got[len(got)-1], "mcp add") {
		t.Fatalf("calls = %q", got)
	}
	if got, err := filepath.Glob(path); err != nil || len(got) != 1 {
		t.Fatalf("settings file %q, %v", got, err)
	}
}

// equalLines compares two string slices.
func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
