package agent

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
)

// The crash tests run this test binary again as a child that runs a
// conversation, then kill it with no chance to clean up. The child reads
// what to do from these variables.
const (
	crashModeEnv   = "SNOW_CRASH_MODE"   // "stream" or "tool"
	crashDirEnv    = "SNOW_CRASH_DIR"    // the folder of the store
	crashFolderEnv = "SNOW_CRASH_FOLDER" // the conversation folder
)

// TestCrashChild is the child of the crash tests. It does nothing in a
// normal run.
func TestCrashChild(t *testing.T) {
	mode := os.Getenv(crashModeEnv)
	if mode == "" {
		t.Skip("runs only as the child of a crash test")
	}
	store, err := convstore.Open(os.Getenv(crashDirEnv))
	if err != nil {
		t.Fatal(err)
	}
	tools := tool.NewRegistry()
	// block never returns: the parent kills the process while it runs.
	block := fnTool{tool.Def{Name: "block"}, func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}
	if err := tools.Register(block); err != nil {
		t.Fatal(err)
	}
	reply := providertest.Reply{Events: []provider.Event{&provider.TextDelta{Text: "Half a rep"}}, Wait: true}
	if mode == "tool" {
		reply = providertest.Reply{Events: []provider.Event{
			&provider.ToolUse{ID: "t1", Name: "block", Input: json.RawMessage(`{}`)},
			&provider.Stop{Reason: provider.StopToolUse},
		}}
	}
	providers := provider.NewRegistry()
	if err := providers.Register(providertest.New("fake", reply)); err != nil {
		t.Fatal(err)
	}
	c, err := Start(Config{Store: store, Providers: providers, Tools: tools, Folder: tool.Folder{Path: os.Getenv(crashFolderEnv)}}, convstore.Meta{Model: testModel})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Send("hello"); err != nil {
		t.Fatal(err)
	}
	// The child waits for the kill. If the parent dies first, its end of
	// stdin closes and the child exits, so it never lingers. The read also
	// keeps the runtime from calling the blocked conversation a deadlock.
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

// lockedBuffer collects the child's output; exec writes it from its own
// goroutine while a failing test reads it.
type lockedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// crash runs the child in mode, waits until a record of type want reaches
// the file, kills the child, and returns the store, the conversation ID,
// and the conversation folder.
func crash(t *testing.T, mode string, want convstore.Type) (*convstore.Store, string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "conversations")
	folder := t.TempDir()
	// os.Args[0] may be a bare name, which exec looks up in PATH.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^TestCrashChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), crashModeEnv+"="+mode, crashDirEnv+"="+dir, crashFolderEnv+"="+folder)
	var out lockedBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close() }()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	killed := false
	t.Cleanup(func() {
		if !killed {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	id := waitRecord(t, dir, want, &out)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill: %v; child output:\n%s", err, out.String())
	}
	// Wait reports the kill as an error; what matters is that it returns,
	// so the process is gone before the file is read.
	_ = cmd.Wait()
	killed = true
	store, err := convstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return store, id, folder
}

// waitRecord polls dir until its one conversation file holds a line of
// type want, and returns the conversation ID.
func waitRecord(t *testing.T, dir string, want convstore.Type, out *lockedBuffer) string {
	t.Helper()
	tag := `{"type":"` + string(want) + `"`
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		if len(files) != 1 {
			continue
		}
		b, err := os.ReadFile(files[0])
		if err == nil && strings.Contains(string(b), tag) {
			return strings.TrimSuffix(filepath.Base(files[0]), ".jsonl")
		}
	}
	t.Fatalf("no %s record reached the file; child output:\n%s", want, out.String())
	return ""
}

func TestCrashMidStream(t *testing.T) {
	store, id, _ := crash(t, "stream", convstore.TypePartial)
	c, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	want := []convstore.Type{convstore.TypeUser, convstore.TypeAssistant}
	if got := types(c.Records); !reflect.DeepEqual(got, want) {
		t.Fatalf("records %v, want %v", got, want)
	}
	if a := c.Records[1].(*convstore.Assistant); !a.CutOff || a.Blocks[0].Text != "Half a rep" {
		t.Fatalf("reply %+v", a)
	}
}

func TestCrashMidTool(t *testing.T) {
	store, id, folder := crash(t, "tool", convstore.TypeAssistant)
	c, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if !c.MidTurn {
		t.Fatal("the load does not show the turn open")
	}
	// The call is saved; the load marks its result lost, so the exchange
	// stays valid for the provider.
	n := len(c.Records)
	a, ok := c.Records[n-2].(*convstore.Assistant)
	lost, ok2 := c.Records[n-1].(*convstore.ToolResult)
	if !ok || !ok2 || a.Blocks[0].Name != "block" || !lost.Lost || lost.ToolUseID != "t1" {
		r, _ := json.Marshal(c.Records)
		t.Fatalf("records %s", r)
	}
	fake := providertest.New("fake")
	providers := provider.NewRegistry()
	if err := providers.Register(fake); err != nil {
		t.Fatal(err)
	}
	conv, _, err := Resume(Config{Store: store, Providers: providers, Tools: tool.NewRegistry(), Folder: tool.Folder{Path: folder}}, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := conv.Close(); err != nil {
		t.Fatal(err)
	}
	c, err = store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.MidTurn || c.Records[len(c.Records)-1].Type() != convstore.TypeInterruption {
		t.Fatalf("after resume: mid-turn %v, records %v", c.MidTurn, types(c.Records))
	}
	if n := len(fake.Requests()); n != 0 {
		t.Fatalf("the provider got %d calls after the load", n)
	}
}

// TestTornLastLine: a file cut in the middle of its last record loads
// every earlier record and drops the cut one.
func TestTornLastLine(t *testing.T) {
	h := newHarness(t, nil, text("First."), text("Second."))
	_ = h.conv.Send("one")
	next[*TurnEnded](t, h)
	_ = h.conv.Send("two")
	next[*TurnEnded](t, h)
	path := h.conv.w.Path()
	if err := h.conv.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	last := strings.LastIndex(strings.TrimSuffix(string(b), "\n"), "\n") + 1
	if err := os.WriteFile(path, b[:last+(len(b)-last)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := h.store.Load(h.conv.ID())
	if err != nil {
		t.Fatal(err)
	}
	want := []convstore.Type{convstore.TypeUser, convstore.TypeAssistant, convstore.TypeUser}
	if got := types(c.Records); !reflect.DeepEqual(got, want) {
		t.Fatalf("records %v, want %v", got, want)
	}
}
