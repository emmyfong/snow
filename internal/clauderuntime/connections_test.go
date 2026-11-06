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
		connections, err := ListConnections(root, tc.distro)
		if err != nil || len(connections) != tc.want {
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
