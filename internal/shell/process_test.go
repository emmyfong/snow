package shell

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProcessProtocolStreams(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var out, diagnostic lockedWriter
	p, err := StartProcess(Launch{Path: exe, Args: []string{"-test.run=^TestProcessProtocolHelper$"},
		Env: []string{"SNOW_PROCESS_HELPER=1"}}, strings.NewReader("first\nsecond\n"), &out, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Stop(time.Second); err != nil {
			t.Error(err)
		}
	})
	select {
	case <-p.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("protocol process did not finish")
	}
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	if got := out.bytes(); !bytes.Equal(got, []byte("reply:first\nreply:second\n")) {
		t.Fatalf("stdout %q", got)
	}
	if got := diagnostic.bytes(); !bytes.Equal(got, []byte("diagnostic\n")) {
		t.Fatalf("stderr %q", got)
	}
}

func TestProcessProtocolHelper(_ *testing.T) {
	if os.Getenv("SNOW_PROCESS_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if _, err := fmt.Fprintln(os.Stdout, "reply:"+scanner.Text()); err != nil {
			os.Exit(2)
		}
	}
	if scanner.Err() != nil {
		os.Exit(3)
	}
	if _, err := fmt.Fprintln(os.Stderr, "diagnostic"); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}
