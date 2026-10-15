package clipboard

import (
	"errors"
	"testing"
	"unsafe"
)

var (
	procGetData = user32.NewProc("GetClipboardData")
	procEnum    = user32.NewProc("EnumClipboardFormats")
	procHas     = user32.NewProc("IsClipboardFormatAvailable")
)

// read returns the clipboard's text and whether it has any contents of
// another kind, which a test must not overwrite.
func read(t *testing.T) (text string, other bool) {
	t.Helper()
	if err := openClipboard(); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _, _ = procClose.Call() }()
	// Windows adds plain-text, OEM-text, and locale formats beside Unicode
	// text by itself; any other format is something a test must not erase.
	for f, _, _ := procEnum.Call(0); f != 0; f, _, _ = procEnum.Call(f) {
		switch f {
		case 1, 7, cfUnicodeText, 16:
		default:
			other = true
		}
	}
	has, _, _ := procHas.Call(cfUnicodeText)
	if has == 0 {
		return "", other
	}
	h, _, err := procGetData.Call(cfUnicodeText)
	if h == 0 {
		t.Fatalf("GetClipboardData: %v", err)
	}
	addr, _, err := procLock.Call(h)
	if addr == 0 {
		t.Fatalf("GlobalLock: %v", err)
	}
	defer func() { _, _, _ = procUnlock.Call(h) }()
	p := *(*unsafe.Pointer)(unsafe.Pointer(&addr))
	var u16 []uint16
	for i := 0; ; i++ {
		c := *(*uint16)(unsafe.Add(p, i*2))
		if c == 0 {
			break
		}
		u16 = append(u16, c)
	}
	return string(utf16Decode(u16)), other
}

func utf16Decode(u []uint16) []rune {
	var out []rune
	for i := 0; i < len(u); i++ {
		r := rune(u[i])
		if r >= 0xD800 && r < 0xDC00 && i+1 < len(u) {
			r = 0x10000 + (r-0xD800)<<10 + (rune(u[i+1]) - 0xDC00)
			i++
		}
		out = append(out, r)
	}
	return out
}

// TestSetWritesTheWindowsClipboard writes text with the command Set returns
// and reads it back. It puts the clipboard's old text back, and skips when
// the clipboard holds anything but text, which it could not restore.
func TestSetWritesTheWindowsClipboard(t *testing.T) {
	before, other := read(t)
	if other {
		t.Skip("the clipboard holds more than text; not overwriting it")
	}
	t.Cleanup(func() {
		if before != "" {
			if err := write(before); err != nil {
				t.Errorf("restore the clipboard: %v", err)
			}
		}
	})
	tests := []struct{ name, text, want string }{
		{"ascii", "hello", "hello"},
		{"line breaks become CRLF", "one\ntwo\r\nthree", "one\r\ntwo\r\nthree"},
		{"wide and astral characters", "漢字 and 😀", "漢字 and 😀"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if msg := Set(tt.text)(); msg != nil {
				t.Fatalf("Set reported %v", msg)
			}
			if got, _ := read(t); got != tt.want {
				t.Fatalf("clipboard holds %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrMsgWrapsItsCause(t *testing.T) {
	cause := errClipboardBusy
	err := error(ErrMsg{Err: cause})
	if !errors.Is(err, cause) {
		t.Fatal("ErrMsg should unwrap to its cause")
	}
}
