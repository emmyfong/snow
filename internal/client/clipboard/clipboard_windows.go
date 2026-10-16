package clipboard

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13  // the clipboard format of UTF-16 text
	gmemMoveable  = 0x2 // GlobalAlloc flag the clipboard requires
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procOpen         = user32.NewProc("OpenClipboard")
	procClose        = user32.NewProc("CloseClipboard")
	procEmpty        = user32.NewProc("EmptyClipboard")
	procSetData      = user32.NewProc("SetClipboardData")
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procAlloc        = kernel32.NewProc("GlobalAlloc")
	procLock         = kernel32.NewProc("GlobalLock")
	procUnlock       = kernel32.NewProc("GlobalUnlock")
	procFree         = kernel32.NewProc("GlobalFree")
	errClipboardBusy = errors.New("another program holds the clipboard")
)

// Set returns a command that puts text on the Windows clipboard. It returns
// an ErrMsg when that fails, and nil when it works.
func Set(text string) tea.Cmd {
	return func() tea.Msg {
		if err := write(text); err != nil {
			return ErrMsg{Err: err}
		}
		return nil
	}
}

// write replaces the clipboard's contents with text. Other programs open the
// clipboard briefly too, so a busy clipboard is retried for a moment.
func write(text string) error {
	// Windows programs expect CRLF line breaks in clipboard text.
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
	u16, err := windows.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("encode text: %w", err)
	}
	if err := openClipboard(); err != nil {
		return err
	}
	defer func() { _, _, _ = procClose.Call() }()
	if r, _, err := procEmpty.Call(); r == 0 {
		return fmt.Errorf("empty clipboard: %w", err)
	}
	size := uintptr(len(u16)) * unsafe.Sizeof(u16[0])
	h, _, err := procAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("allocate clipboard memory: %w", err)
	}
	addr, _, err := procLock.Call(h)
	if addr == 0 {
		_, _, _ = procFree.Call(h)
		return fmt.Errorf("lock clipboard memory: %w", err)
	}
	// The address comes from the system, not from Go memory. Reading it
	// through a pointer-sized variable keeps go vet's unsafeptr check quiet
	// without hiding a real misuse.
	p := *(*unsafe.Pointer)(unsafe.Pointer(&addr))
	copy(unsafe.Slice((*uint16)(p), len(u16)), u16)
	// GlobalUnlock reports an error when the lock count reaches zero, which
	// is the success case here.
	_, _, _ = procUnlock.Call(h)
	if r, _, err := procSetData.Call(cfUnicodeText, h); r == 0 {
		_, _, _ = procFree.Call(h) // the system owns the memory only after a successful SetClipboardData
		return fmt.Errorf("set clipboard data: %w", err)
	}
	return nil
}

func openClipboard() error {
	for range 20 {
		if r, _, _ := procOpen.Call(0); r != 0 {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("open clipboard: %w", errClipboardBusy)
}
