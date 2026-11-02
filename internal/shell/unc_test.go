package shell

import "testing"

func TestWSLFolderPath(t *testing.T) {
	if got := WSLToUNC("Ubuntu", "/home/u"); got != `\\wsl.localhost\Ubuntu\home\u` {
		t.Errorf("WSLToUNC = %s", got)
	}
	tests := []struct {
		in, distro, linux string
		ok                bool
	}{
		{`\\wsl.localhost\Ubuntu\home\u`, "Ubuntu", "/home/u", true},
		{`\\WSL$\Debian\`, "Debian", "/", true},
		{`\\wsl.localhost\`, "", "", false},
		{`C:\Users\u`, "", "", false},
	}
	for _, tt := range tests {
		d, l, ok := UNCToWSL(tt.in)
		if d != tt.distro || l != tt.linux || ok != tt.ok {
			t.Errorf("UNCToWSL(%q) = %q %q %v", tt.in, d, l, ok)
		}
	}
}
