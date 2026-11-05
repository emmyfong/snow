package main

import (
	"reflect"
	"testing"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/server"
)

// TestRoleOptions: roles come in name order, with their titles.
func TestRoleOptions(t *testing.T) {
	cfg := config.Config{Roles: map[string]config.Role{
		"reviewer": {Title: "Reviewer", Instructions: "Review.", PermissionMode: "plan"},
		"coder":    {Instructions: "Code."},
	}}
	roles, dir, err := roleOptions(cfg)
	if err != nil || dir == "" {
		t.Fatalf("dir %q, err %v", dir, err)
	}
	want := []server.Role{{Name: "coder", Instructions: "Code."}, {Name: "Reviewer", Instructions: "Review.", PermissionMode: "plan"}}
	if !reflect.DeepEqual(roles, want) {
		t.Fatalf("roles %+v, want %+v", roles, want)
	}
}
