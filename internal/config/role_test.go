package config

import (
	"errors"
	"reflect"
	"testing"
)

func TestConfigRoles(t *testing.T) {
	tests := []struct {
		name string
		body string
		line int    // of the error; 0 means the file loads
		key  string // of the error
	}{
		{"valid", `
[role.reviewer]
title = "Reviewer"
instructions = "Review the diff. Do not edit files."
permission_mode = "plan"

[role.coder]
instructions = "Fix the bug and add a test."
`, 0, ""},
		{"empty", `
[role.reviewer]
title = "Reviewer"
`, 2, "role.reviewer"},
		{"unknown mode", `
[role.reviewer]
permission_mode = "readonly"
`, 3, "role.reviewer.permission_mode"},
		{"bypass refused", `
[role.yolo]
permission_mode = "bypassPermissions"
`, 3, "role.yolo.permission_mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tt.body))
			if tt.line == 0 {
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]Role{
					"reviewer": {Title: "Reviewer", Instructions: "Review the diff. Do not edit files.", PermissionMode: "plan"},
					"coder":    {Instructions: "Fix the bug and add a test."},
				}
				if !reflect.DeepEqual(cfg.Roles, want) {
					t.Fatalf("roles %+v, want %+v", cfg.Roles, want)
				}
				return
			}
			var ce *Error
			if !errors.As(err, &ce) || ce.Line != tt.line || ce.Key != tt.key {
				t.Fatalf("error %v (%+v), want line %d key %s", err, ce, tt.line, tt.key)
			}
		})
	}
}
