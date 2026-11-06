package clauderuntime

import "testing"

// TestStatusFolderIgnoresCaseOnWindows: Claude may report its folder in
// another case on Windows; it is still this connection's folder.
func TestStatusFolderIgnoresCaseOnWindows(t *testing.T) {
	p := Profile{ID: "c1", ConfigDir: `C:\Users\U\AppData\Local\snow\runtimes\connections\c1\claude`}
	if !p.configDirMatches(`c:\users\u\appdata\local\snow\runtimes\connections\c1\claude`) {
		t.Fatal("same folder in another case did not match")
	}
	if p.configDirMatches(`C:\Users\U\.claude`) {
		t.Fatal("another folder matched")
	}
}
