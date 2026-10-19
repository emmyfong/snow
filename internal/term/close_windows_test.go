package term

import (
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestCancelReadWakesBlockedRead: ConPTY's output is a synchronous
// anonymous pipe, and closing its handle does not wake a ReadFile blocked on
// it. cancelRead must.
func TestCancelReadWakesBlockedRead(t *testing.T) {
	var r, w windows.Handle
	if err := windows.CreatePipe(&r, &w, nil, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(w) }()
	defer func() { _ = windows.CloseHandle(r) }()
	done := make(chan error, 1)
	go func() {
		var n uint32
		buf := make([]byte, 16)
		done <- windows.ReadFile(r, buf, &n, nil)
	}()
	time.Sleep(200 * time.Millisecond) // let the read block
	cancelRead(uintptr(r))
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the read returned no error, want it cancelled")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelRead did not wake the blocked read")
	}
}
