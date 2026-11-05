package main

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

// sleepEnv makes TestHelperSleep keep a copy of the test binary running, as
// a Snow server keeps snow.exe running while an upgrade replaces it.
const sleepEnv = "SNOW_TEST_SLEEP"

func TestHelperSleep(t *testing.T) {
	if os.Getenv(sleepEnv) != "1" {
		t.Skip("helper process")
	}
	time.Sleep(time.Minute)
}

// fakeWindowsRelease writes a release folder with snow_windows_<arch>.zip,
// holding this test binary as snow.exe (asSnow makes it act as snow), and
// checksums.txt. good=false writes a checksum that does not match.
func fakeWindowsRelease(t *testing.T, good bool) string {
	t.Helper()
	dir := t.TempDir()
	name := "snow_windows_" + runtime.GOARCH + ".zip"
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()
	out, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	w, err := zw.Create("snow.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(w, src); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	hexSum := hex.EncodeToString(sum[:])
	if !good {
		hexSum = strings.Repeat("0", len(hexSum))
	}
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(hexSum+"  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// shells are the PowerShells to run install.ps1 under: Windows PowerShell
// 5.1, built into Windows, and PowerShell 7 when it is installed.
func shells(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, s := range []string{"powershell.exe", "pwsh.exe"} {
		if _, err := exec.LookPath(s); err == nil {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		t.Skip("no PowerShell")
	}
	return out
}

// runInstallPS runs install.ps1 in shell against release into dest. With
// iex set it runs the script as irm | iex does, inside the session. extra
// adds environment; PATH is left alone unless extra names SNOW_PATH_KEY.
func runInstallPS(t *testing.T, shell, release, dest string, iex bool, extra ...string) (string, error) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script}
	if iex {
		args = []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
			"Get-Content -Raw -LiteralPath '" + script + "' | Invoke-Expression"}
	}
	cmd := exec.Command(shell, args...)
	cmd.Env = append(os.Environ(), "SNOW_BASE_URL="+release, "SNOW_INSTALL_DIR="+dest, asSnow+"=1")
	if !strings.Contains(strings.Join(extra, " "), "SNOW_PATH_KEY=") {
		cmd.Env = append(cmd.Env, "SNOW_NO_PATH=1")
	}
	cmd.Env = append(cmd.Env, extra...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInstallScriptWindows(t *testing.T) {
	release := fakeWindowsRelease(t, true)
	for _, shell := range shells(t) {
		for _, iex := range []bool{false, true} {
			t.Run(shell+map[bool]string{false: " file", true: " iex"}[iex], func(t *testing.T) {
				dest := filepath.Join(t.TempDir(), "snow")
				out, err := runInstallPS(t, shell, release, dest, iex)
				if err != nil {
					t.Fatalf("install.ps1: %v\n%s", err, out)
				}
				if !strings.Contains(out, "Installed snow") {
					t.Fatalf("output %q, want the installed version", out)
				}
				entries, err := os.ReadDir(dest)
				if err != nil || len(entries) != 1 || entries[0].Name() != "snow.exe" {
					t.Fatalf("%s holds %v (%v), want snow.exe alone", dest, entries, err)
				}
			})
		}
	}
}

// TestInstallScriptWindowsFromPwsh: Windows PowerShell started from
// PowerShell 7 inherits its PSModulePath and cannot load Get-FileHash or
// Expand-Archive; the install must still work, and fail with a nonzero exit
// code when it does not.
func TestInstallScriptWindowsFromPwsh(t *testing.T) {
	if _, err := exec.LookPath("pwsh.exe"); err != nil {
		t.Skip("no PowerShell 7")
	}
	modulePath, err := exec.Command("pwsh.exe", "-NoProfile", "-Command", "$env:PSModulePath").Output()
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "snow")
	out, err := runInstallPS(t, "powershell.exe", fakeWindowsRelease(t, true), dest, false,
		"PSModulePath="+strings.TrimSpace(string(modulePath)))
	if err != nil || !strings.Contains(out, "Installed snow") {
		t.Fatalf("install.ps1 = %v\n%s\nwant snow installed", err, out)
	}
}

// TestInstallScriptWindowsRefusesBadChecksum: a download that does not
// match the release's checksum is never installed.
func TestInstallScriptWindowsRefusesBadChecksum(t *testing.T) {
	release := fakeWindowsRelease(t, false)
	for _, shell := range shells(t) {
		dest := filepath.Join(t.TempDir(), "snow")
		out, err := runInstallPS(t, shell, release, dest, false)
		if err == nil || !strings.Contains(out, "checksum mismatch") {
			t.Fatalf("%s: install.ps1 = %v\n%s\nwant a checksum mismatch", shell, err, out)
		}
		if _, err := os.Stat(filepath.Join(dest, "snow.exe")); !os.IsNotExist(err) {
			t.Fatalf("%s: snow.exe was installed despite the bad checksum", shell)
		}
	}
}

// TestInstallScriptWindowsUpgradesRunningSnow: an upgrade replaces snow.exe
// while a Snow server still runs it, as it normally does.
func TestInstallScriptWindowsUpgradesRunningSnow(t *testing.T) {
	release := fakeWindowsRelease(t, true)
	dest := filepath.Join(t.TempDir(), "snow")
	out, err := runInstallPS(t, "powershell.exe", release, dest, false)
	if err != nil {
		t.Fatalf("first install: %v\n%s", err, out)
	}
	running := exec.Command(filepath.Join(dest, "snow.exe"), "-test.run=^TestHelperSleep$")
	running.Env = append(os.Environ(), sleepEnv+"=1")
	if err := running.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = running.Process.Kill(); _ = running.Wait() })
	time.Sleep(500 * time.Millisecond)

	// Twice: the first moves the running file aside, and the server keeps
	// it open, so the second must not need that name again.
	for i := range 2 {
		out, err = runInstallPS(t, "powershell.exe", release, dest, false)
		if err != nil {
			t.Fatalf("upgrade %d while snow.exe runs: %v\n%s", i+1, err, out)
		}
		if !strings.Contains(out, "Installed snow") {
			t.Fatalf("output %q, want the installed version", out)
		}
	}
}

// TestInstallScriptWindowsKeepsPathEntries: the PATH edit adds the folder
// once, keeps entries such as %USERPROFILE%\bin as written, and keeps the
// value expandable. It runs on a scratch registry key, not the user's PATH.
func TestInstallScriptWindowsKeepsPathEntries(t *testing.T) {
	before := `%USERPROFILE%\bin;C:\Tools`
	keyName := testPathKey(t, before)

	release := fakeWindowsRelease(t, true)
	dest := filepath.Join(t.TempDir(), "snow")
	for range 2 { // the second run must not add the folder again
		if out, err := runInstallPS(t, "powershell.exe", release, dest, false, "SNOW_PATH_KEY="+keyName); err != nil {
			t.Fatalf("install.ps1: %v\n%s", err, out)
		}
	}
	got, kind := readPathKey(t, keyName)
	added, ok := strings.CutPrefix(got, before+";")
	if kind != registry.EXPAND_SZ || !ok || strings.Contains(added, ";") {
		t.Fatalf("Path = %q (kind %d), want %q plus one entry, as an expandable string", got, kind, before)
	}
	if !sameDir(added, dest) {
		t.Fatalf("added PATH entry %q is not %q", added, dest)
	}
}

// testPathKey makes a scratch registry key under HKCU with Path set to
// path, for SNOW_PATH_KEY, so a test never edits the user's own PATH.
func testPathKey(t *testing.T, path string) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	keyName := `Software\SnowInstallTest` + hex.EncodeToString(b)
	k, _, err := registry.CreateKey(registry.CURRENT_USER, keyName, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, keyName) })
	defer func() { _ = k.Close() }()
	if err := k.SetExpandStringValue("Path", path); err != nil {
		t.Fatal(err)
	}
	return keyName
}

// readPathKey returns the Path value of a key from testPathKey, and its
// registry kind.
func readPathKey(t *testing.T, keyName string) (string, uint32) {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, keyName, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()
	got, kind, err := k.GetStringValue("Path")
	if err != nil {
		t.Fatal(err)
	}
	return got, kind
}

// sameDir reports whether a and b name one folder. A PATH entry may spell
// it differently: a temp folder can be an 8.3 short name (RUNNER~1) that
// the script writes out in full.
func sameDir(a, b string) bool {
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(fa, fb)
}

// TestInstallPSFromSource: with SNOW_SOURCE, install.ps1 builds the
// checkout, installs that build, and adds its folder to PATH; a checkout
// that does not build installs nothing.
func TestInstallPSFromSource(t *testing.T) {
	good, broken := fakeCheckout(t, false), fakeCheckout(t, true)
	for _, shell := range shells(t) {
		t.Run(shell, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "snow")
			key := testPathKey(t, `C:\Tools`)
			out, err := runInstallPS(t, shell, "", dest, false, "SNOW_SOURCE="+good, "SNOW_PATH_KEY="+key, "GOFLAGS=", "GOWORK=off")
			if err != nil {
				t.Fatalf("install.ps1: %v\n%s", err, out)
			}
			if !strings.Contains(out, "Installed snow dev-fake") || !strings.Contains(out, "not installing a release") || strings.Contains(out, "Downloading") {
				t.Fatalf("output %q, want the source build named, installed, and nothing downloaded", out)
			}
			path, _ := readPathKey(t, key)
			if added, ok := strings.CutPrefix(path, `C:\Tools;`); !ok || !sameDir(added, dest) {
				t.Errorf("PATH %q, want %s added", path, dest)
			}

			// "." is the checkout PowerShell's location is in, as the README
			// tells the user to run it.
			dest = filepath.Join(t.TempDir(), "snow")
			script, err := filepath.Abs(filepath.Join("..", "..", "install.ps1"))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(shell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
				"Set-Location -LiteralPath '"+good+"'; & '"+script+"'")
			cmd.Env = append(os.Environ(), "SNOW_SOURCE=.", "SNOW_INSTALL_DIR="+dest, "SNOW_NO_PATH=1", "GOFLAGS=", "GOWORK=off")
			if b, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(b), "Installed snow dev-fake") {
				t.Fatalf("SNOW_SOURCE=. after Set-Location: %v\n%s", err, b)
			}

			dest = filepath.Join(t.TempDir(), "snow")
			out, err = runInstallPS(t, shell, "", dest, false, "SNOW_SOURCE="+broken, "GOFLAGS=", "GOWORK=off")
			if err == nil || !strings.Contains(out, "go build failed") {
				t.Fatalf("install.ps1 = %v\n%s\nwant the build failure", err, out)
			}
			if _, err := os.Stat(filepath.Join(dest, "snow.exe")); !os.IsNotExist(err) {
				t.Fatal("snow.exe was installed from a failed build")
			}
		})
	}
}
