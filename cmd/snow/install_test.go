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
	cmd.Env = append(os.Environ(), "SNOW_BASE_URL=file://"+release, "SNOW_INSTALL_DIR="+bin)
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
