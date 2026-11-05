package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeRelease writes a release folder with snow_<os>_<arch>.tar.gz, holding a
// stand-in snow that prints a version, and checksums.txt. good=false writes
// a checksum that does not match the archive.
func fakeRelease(t *testing.T, good bool) string {
	t.Helper()
	dir := t.TempDir()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho snow 9.9.9\n")
	if err := tw.WriteHeader(&tar.Header{Name: "snow", Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	name := "snow_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	hexSum := hex.EncodeToString(sum[:])
	if !good {
		hexSum = strings.Repeat("0", len(hexSum))
	}
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(hexSum+"  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runInstall runs install.sh against release and returns where it
// installs, its output, and its error.
func runInstall(t *testing.T, release string) (string, string, error) {
	t.Helper()
	return runInstallEnv(t, "SNOW_BASE_URL=file://"+release)
}

// runInstallEnv runs install.sh with env into a new folder and returns
// that folder, the output, and the error. The user's GOFLAGS and go.work
// stay out of a build from source.
func runInstallEnv(t *testing.T, env ...string) (string, string, error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is for Linux and WSL")
	}
	for _, tool := range []string{"sh", "curl", "tar"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	bin := filepath.Join(t.TempDir(), "bin")
	cmd := exec.Command("sh", filepath.Join("..", "..", "install.sh"))
	cmd.Env = append(os.Environ(), "SNOW_INSTALL_DIR="+bin, "GOFLAGS=", "GOWORK=off")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	return bin, string(out), err
}

func TestInstallScript(t *testing.T) {
	bin, out, err := runInstall(t, fakeRelease(t, true))
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Installed snow 9.9.9") {
		t.Fatalf("output %q, want the installed version", out)
	}
	if fi, err := os.Stat(filepath.Join(bin, "snow")); err != nil || fi.Mode()&0o111 == 0 {
		t.Fatalf("snow in %s: %v, want an executable", bin, err)
	}
}

// TestInstallScriptRefusesBadChecksum: a download that does not match the
// release's checksum is never installed.
func TestInstallScriptRefusesBadChecksum(t *testing.T) {
	bin, out, err := runInstall(t, fakeRelease(t, false))
	if err == nil {
		t.Fatalf("install.sh installed a download with a bad checksum:\n%s", out)
	}
	if !strings.Contains(out, "checksum mismatch") {
		t.Fatalf("output %q, want it to name the checksum mismatch", out)
	}
	if _, err := os.Stat(filepath.Join(bin, "snow")); !os.IsNotExist(err) {
		t.Fatal("snow was installed despite the bad checksum")
	}
}

// fakeCheckout writes a Go module laid out as a Snow checkout, whose
// cmd/snow prints "snow dev-fake" for snow version. broken=true writes one
// that does not compile. The real dev-<hash> version is buildversion.go's,
// tested in buildversion_test.go.
func fakeCheckout(t *testing.T, broken bool) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	dir := t.TempDir()
	main := "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tif len(os.Args) > 1 && os.Args[1] == \"version\" {\n\t\tfmt.Println(\"snow dev-fake\")\n\t}\n}\n"
	if broken {
		main = "package main\n\nfunc main() { this is not go }\n"
	}
	files := map[string]string{"go.mod": "module fakesnow\n\ngo 1.22\n", "cmd/snow/main.go": main}
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestInstallFromSource: with SNOW_SOURCE, install.sh builds the checkout
// and installs that build, with no release download.
func TestInstallFromSource(t *testing.T) {
	bin, out, err := runInstallEnv(t, "SNOW_SOURCE="+fakeCheckout(t, false))
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Installed snow dev-fake") || !strings.Contains(out, "not installing a release") || strings.Contains(out, "Downloading") {
		t.Fatalf("output %q, want the source build named, installed, and nothing downloaded", out)
	}
	if fi, err := os.Stat(filepath.Join(bin, "snow")); err != nil || fi.Mode()&0o111 == 0 {
		t.Fatalf("snow in %s: %v, want an executable", bin, err)
	}
}

// TestInstallFromSourceBuildFails: a checkout that does not build installs
// nothing.
func TestInstallFromSourceBuildFails(t *testing.T) {
	bin, out, err := runInstallEnv(t, "SNOW_SOURCE="+fakeCheckout(t, true))
	if err == nil || !strings.Contains(out, "go build failed") {
		t.Fatalf("install.sh = %v\n%s\nwant the build failure", err, out)
	}
	if _, err := os.Stat(filepath.Join(bin, "snow")); !os.IsNotExist(err) {
		t.Fatal("snow was installed from a failed build")
	}
}
