package shell

import "testing"

func TestDecodeUTF16LE(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"text", []byte{'U', 0, 'b', 0, 'u', 0}, "Ubu"},
		{"byte order mark", []byte{0xFF, 0xFE, 'a', 0}, "a"},
		{"outside the basic plane", []byte{0x3D, 0xD8, 0x00, 0xDE}, "\U0001F600"},
		{"a lone byte at the end is dropped", []byte{'a', 0, 'b'}, "a"},
		{"empty", nil, ""},
	}
	for _, tt := range tests {
		if got := DecodeUTF16LE(tt.in); got != tt.want {
			t.Errorf("%s: DecodeUTF16LE = %q, want %q", tt.name, got, tt.want)
		}
	}
}
