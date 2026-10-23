package credential

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestKeyNeverInError: every failure leaves the key out of its text.
func TestKeyNeverInError(t *testing.T) {
	bad := secret + " tail"
	cases := map[string]func(t *testing.T) error{
		"bad key in env": func(*testing.T) error {
			_, _, err := newStore(env(map[string]string{"ANTHROPIC_API_KEY": bad}), newFakeOS(), t.TempDir(), nil).Get("anthropic")
			return err
		},
		"bad key in OS store": func(*testing.T) error {
			o := newFakeOS()
			o.keys["anthropic"] = bad
			_, _, err := newStore(env(nil), o, t.TempDir(), nil).Get("anthropic")
			return err
		},
		"bad file line": func(t *testing.T) error {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, fileName), []byte(secret+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, _, err := newStore(env(nil), newFakeOS(), dir, nil).Get("anthropic")
			return err
		},
		"OS store refuses": func(t *testing.T) error {
			o := newFakeOS()
			o.failSet = true
			_, err := newStore(env(nil), o, t.TempDir(), nil).Set("anthropic", mustKey(t, secret))
			return err
		},
		"file cannot be written": func(t *testing.T) error {
			dir := filepath.Join(t.TempDir(), "taken")
			if err := os.WriteFile(dir, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			o := newFakeOS()
			o.broken = true
			_, err := newStore(env(nil), o, dir, nil).Set("anthropic", mustKey(t, secret))
			return err
		},
		"bad new key": func(*testing.T) error {
			_, err := NewKey(bad)
			return err
		},
	}
	for name, run := range cases {
		err := run(t)
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(fmt.Sprintf("%+v", err), secret) {
			t.Errorf("%s: error holds the key: %v", name, err)
		}
	}
}

// TestStringHidesKey: no fmt verb prints the key.
func TestStringHidesKey(t *testing.T) {
	k := mustKey(t, secret)
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d"} {
		if got := fmt.Sprintf(verb, k); strings.Contains(got, secret) || got != hidden {
			t.Errorf("%s printed %q", verb, got)
		}
	}
	if got := fmt.Sprintf("%v", struct{ K Key }{k}); strings.Contains(got, secret) {
		t.Errorf("a struct holding the key printed %q", got)
	}
}
