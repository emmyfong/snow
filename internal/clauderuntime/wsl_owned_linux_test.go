package clauderuntime

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

// ownedWSLFixture simulates wsl.exe's argv and distro home using real sh
// processes. The Claude executable is Snow's model-free protocol fixture.
func ownedWSLFixture(t *testing.T, refuseReceipt bool) (Profile, shell.Profile) {
	t.Helper()
	home, bin := t.TempDir(), t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(text string) string { return "'" + strings.ReplaceAll(text, "'", `'"'"'`) + "'" }
	script := "#!/bin/sh\nexport HOME=" + quote(home) + "\nexport SHELL=/bin/sh\nexport PATH=" + quote(bin+":/usr/bin:/bin") + "\n"
	if refuseReceipt {
		script += `case "$*" in *snow-receipt*) exit 78 ;; esac` + "\n"
	}
	script += `while [ "$1" != -e ]; do shift; done
shift
exec "$@"
`
	bridge := filepath.Join(bin, "wsl")
	if err := os.WriteFile(bridge, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	fixture := "#!/bin/sh\nexport SNOW_CLAUDE_STREAM_TEST=conversation-stop\nexec " + quote(executable) + " -test.run='^TestStreamHelper$' -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fixture), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".profile", ".bash_profile", ".zprofile"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("export PATH="+quote(bin+":/usr/bin:/bin")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	profile, err := ProfileFor(t.TempDir(), Connection{ID: "selected", Distro: "Test", Auth: Subscription})
	if err != nil {
		t.Fatal(err)
	}
	return profile, shell.Profile{Path: bridge, Args: []string{"-d", "Test"}, Distro: "Test"}
}

func TestWSLOwnedStreamCleanup(t *testing.T) {
	profile, program := ownedWSLFixture(t, false)
	launch, err := profile.ChatLaunch(program, "", "", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := profile.Connect(ctx, program, launch)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Stop(); err != nil && !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if errors.Is(client.Err(), ErrStop) {
		t.Fatalf("verified Linux cleanup remained uncertain: %v", client.Err())
	}
}

func TestWSLOwnedNaturalRuntimeExitStopsInheritedOutputChild(t *testing.T) {
	run := t.TempDir()
	marker := filepath.Join(run, "child.pid")
	relayMarker := filepath.Join(run, "relay.pid")
	bin := t.TempDir()
	// Record the relay after its shell has replaced itself with cat.
	wrapper := "#!/bin/sh\nif [ \"$#\" -eq 0 ]; then echo $$ > \"" + relayMarker + "\"; fi\nexec /bin/cat \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "cat"), []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	input, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := input.Close(); err != nil {
			t.Error(err)
		}
		if err := inputWriter.Close(); err != nil {
			t.Error(err)
		}
	})
	output, outputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := output.Close(); err != nil {
			t.Error(err)
		}
	})
	cmd := exec.Command("/bin/sh", "-c", wslOwnedScript, "snow-owned", run,
		"/bin/sh", "-c", `sleep 30 & echo $! > "$1"; exit 0`, "fixture", marker)
	cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, outputWriter, os.Stderr
	if err := cmd.Start(); err != nil {
		if closeErr := outputWriter.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	t.Cleanup(func() {
		// The runtime has its own session. Reap both groups even after an assertion fails.
		for _, path := range []string{filepath.Join(run, "leader"), marker, relayMarker} {
			data, err := os.ReadFile(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				t.Error(err)
				continue
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Error(err)
				continue
			}
			target := pid
			if filepath.Base(path) == "leader" {
				target = -pid
			}
			if err := syscall.Kill(target, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
				t.Error(err)
			}
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Error(err)
		}
	})
	if err := outputWriter.Close(); err != nil {
		t.Fatal(err)
	}
	drained := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, output); drained <- err }()
	select {
	case err := <-waited:
		if err != nil {
			t.Fatalf("supervisor failed after runtime exit: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("natural runtime exit did not finish cleanup while stdin stayed open")
	}
	receipt, err := os.ReadFile(filepath.Join(run, "receipt"))
	if err != nil || string(receipt) != "stopped\n" {
		t.Fatalf("unexpected cleanup receipt %q, %v", receipt, err)
	}
	for _, path := range []string{marker, relayMarker} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			t.Fatal(err)
		}
		states, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		var exitErr *exec.ExitError
		if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
			t.Fatal(err)
		}
		if state := strings.TrimSpace(string(states)); state != "" && !strings.HasPrefix(state, "Z") {
			t.Errorf("owned process %d remains alive: %s", pid, state)
		}
	}
	select {
	case err := <-drained:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("owned processes kept stdout open after cleanup")
	}
}

func TestWSLOwnedCleanupFailureIsNotHiddenByClosedStream(t *testing.T) {
	profile, program := ownedWSLFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := profile.Models(ctx, program, "")
	if !errors.Is(err, ErrStop) {
		t.Fatalf("model probe hid unverified Linux cleanup: %v", err)
	}
}

func TestWSLOwnedNaturalClosePreservesCleanupFailure(t *testing.T) {
	profile, program := ownedWSLFixture(t, true)
	launch, err := profile.ChatLaunch(program, "", "", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := profile.startStream(program, launch)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stream.Stop(); err != nil && !errors.Is(err, ErrStop) && !errors.Is(err, ErrClosed) {
			t.Errorf("stop fixture stream: %v", err)
		}
	})
	// EOF makes the real fixture exit before Stop, so the stream retains both
	// its natural closure and the refused WSL cleanup receipt.
	if err := stream.input.Close(); err != nil {
		t.Fatal(err)
	}
	waitStream(t, stream)
	if !errors.Is(stream.Err(), ErrClosed) || !errors.Is(stream.Err(), ErrStop) {
		t.Fatalf("natural closure did not retain cleanup failure: %v", stream.Err())
	}
	if err := modelStopError(stream.Err()); !errors.Is(err, ErrStop) {
		t.Fatalf("model cleanup hid the real stream's stop failure: %v", err)
	}
}

func TestWSLOwnedMissingUtilityStartsNoRuntime(t *testing.T) {
	run := t.TempDir()
	marker := filepath.Join(run, "runtime-started")
	cmd := exec.Command("/bin/sh", "-c", wslOwnedScript, "snow-owned", run, "/bin/sh", "-c", `touch "$1"`, "fixture", marker)
	cmd.Env = []string{"PATH=" + t.TempDir()}
	if err := cmd.Run(); err == nil {
		t.Fatal("missing utilities were accepted")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("runtime launched without verified ownership: %v", err)
	}
	receipt, err := os.ReadFile(filepath.Join(run, "receipt"))
	if err != nil || string(receipt) != "not-started\n" {
		t.Fatalf("missing startup failure receipt: %q, %v", receipt, err)
	}
}

func TestWSLOwnedLaunchIdentity(t *testing.T) {
	profile, program := ownedWSLFixture(t, false)
	launch, err := profile.ChatLaunch(program, "", "", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	owned, receipt, err := profile.ownedWSLLaunch(program, launch, "run-one")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(owned.Args, " "), "runs/run-one") || receipt.Args[len(receipt.Args)-1] != "run-one" {
		t.Fatal("launch and receipt have different generations")
	}
	if _, _, err := profile.ownedWSLLaunch(program, launch, "../another"); !errors.Is(err, ErrProfile) {
		t.Fatalf("invalid run identity accepted: %v", err)
	}
	foreign := profile
	foreign.ID = "different"
	if _, _, err := foreign.ownedWSLLaunch(program, launch, "run-one"); !errors.Is(err, ErrProfile) {
		t.Fatalf("foreign connection accepted: %v", err)
	}
}
