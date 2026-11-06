package clauderuntime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionsKeepAccountsAndEnvironmentsSeparate(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		distro string
		auth   AuthMode
	}{
		{"", Subscription}, {"", APIKey}, {"Ubuntu", Subscription}, {"Debian", Subscription},
	} {
		connection, err := CreateConnection(root, tc.distro, tc.auth)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadConnection(root, connection.ID)
		if err != nil || loaded != connection {
			t.Fatalf("connection changed after save: %+v, %v", loaded, err)
		}
	}
	for _, tc := range []struct {
		distro string
		want   int
	}{{"", 2}, {"Ubuntu", 1}, {"Debian", 1}, {"missing", 0}} {
		connections, unreadable, err := ListConnections(root, tc.distro)
		if err != nil || len(connections) != tc.want || len(unreadable) != 0 {
			t.Fatalf("environment %q: %d connections, %v", tc.distro, len(connections), err)
		}
		for _, connection := range connections {
			if connection.Distro != tc.distro {
				t.Fatal("offered another environment's connection")
			}
		}
	}
	if _, err := LoadConnection(root, "../standalone"); !errors.Is(err, ErrProfile) {
		t.Fatalf("accepted escaped connection identity: %v", err)
	}
}

func TestConnectionCannotRedirectItsIdentity(t *testing.T) {
	root := t.TempDir()
	connection, err := CreateConnection(root, "", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "connections", connection.ID, "connection.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"id":"other-account","auth":"subscription"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConnection(root, connection.ID); !errors.Is(err, ErrProfile) {
		t.Fatalf("changed connection identity accepted: %v", err)
	}
}

// TestBrokenConnectionHidesNoOther: a connection whose saved file Snow
// cannot read is reported by its ID, and every readable one still lists.
func TestBrokenConnectionHidesNoOther(t *testing.T) {
	root := t.TempDir()
	good, err := CreateConnection(root, "", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "connections", "broken1")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "connection.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	connections, unreadable, err := ListConnections(root, "")
	if err != nil || len(connections) != 1 || connections[0].ID != good.ID {
		t.Fatalf("readable connections: %+v, %v", connections, err)
	}
	if len(unreadable) != 1 || unreadable[0].ID != "broken1" || unreadable[0].Err == nil {
		t.Fatalf("unreadable connections: %+v", unreadable)
	}
}
