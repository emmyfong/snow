package shell

import (
	"errors"
	"strings"
	"testing"
)

// TestClaudeDistrosReportsSilent: snow claude setup learns which
// distribution gave no answer, and still gets the others.
func TestClaudeDistrosReportsSilent(t *testing.T) {
	sys := system{
		goos:     "windows",
		lookPath: func(name string) (string, error) { return `C:\bin\` + name, nil },
		wslList:  func() ([]byte, error) { return utf16le("Ubuntu\r\nDebian\r\n", false), nil },
		claudeIn: func(_, distro string) (bool, error) {
			if distro == "Ubuntu" {
				return false, errors.New("ask Ubuntu for claude: context deadline exceeded")
			}
			return true, nil
		},
	}
	got, err := claudeDistros(sys)
	if len(got) != 1 || got[0].Distro != "Debian" {
		t.Fatalf("claudeDistros() = %+v, want Debian", got)
	}
	if err == nil || !strings.Contains(err.Error(), "Ubuntu") {
		t.Fatalf("error %v, want one that names Ubuntu", err)
	}
}
