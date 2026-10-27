package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestConfigAgents(t *testing.T) {
	tests := []struct {
		name string
		body string
		line int    // of the error; 0 means the file loads
		key  string // of the error
	}{
		{"valid", `
[agent.claude-free]
title = "Claude Code (free)"
command = ["claude"]
env = { ANTHROPIC_BASE_URL = "http://localhost:20128", ANTHROPIC_AUTH_TOKEN = "$secret:omniroute", ANTHROPIC_API_KEY = "" }

[agent.aider]
command = ["aider", "--no-auto-commits"]

[agent.aider.env]
EDITOR = "vim"
`, 0, ""},
		{"no command", `
[agent.claude-free]
title = "Claude Code (free)"
`, 2, "agent.claude-free.command"},
		{"empty command", `
[agent.claude-free]
command = [" "]
`, 3, "agent.claude-free.command"},
		{"bad env value", `
[agent.claude-free]
command = ["claude"]
env = { DEBUG = 1 }
`, 4, "agent.claude-free.env.DEBUG"},
		{"bad env name", `
[agent.claude-free]
command = ["claude"]

[agent.claude-free.env]
"MY-VAR" = "x"
`, 6, "agent.claude-free.env.MY-VAR"},
		{"reserved env name", `
[agent.claude-free]
command = ["claude"]
env = { SNOW_PANE = "3" }
`, 4, "agent.claude-free.env.SNOW_PANE"},
		{"literal token", `
[agent.claude-free]
command = ["claude"]

[agent.claude-free.env]
ANTHROPIC_BASE_URL = "http://localhost:20128"
ANTHROPIC_AUTH_TOKEN = "sk-secret"
`, 7, "agent.claude-free.env.ANTHROPIC_AUTH_TOKEN"},
		{"literal password in lower case", `
[agent.x]
command = ["x"]
env = { db_password = "sk-secret" }
`, 4, "agent.x.env.db_password"},
		{"bad secret name", `
[agent.claude-free]
command = ["claude"]
env = { ANTHROPIC_AUTH_TOKEN = "$secret:Omni Route" }
`, 4, "agent.claude-free.env.ANTHROPIC_AUTH_TOKEN"},
		{"base URL with no token", `
[agent.claude-free]
command = ["claude"]

[agent.claude-free.env]
ANTHROPIC_BASE_URL = "http://localhost:20128"
`, 6, "agent.claude-free.env.ANTHROPIC_BASE_URL"},
		{"unknown key", `
[agent.claude-free]
command = ["claude"]
model = "opus"
`, 4, "agent.claude-free.model"},
		{"bad name", `
[agent.Claude]
command = ["claude"]
`, 2, "agent.Claude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tt.body))
			if tt.line == 0 {
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]Agent{
					"claude-free": {Title: "Claude Code (free)", Command: []string{"claude"}, Env: map[string]string{
						"ANTHROPIC_BASE_URL": "http://localhost:20128", "ANTHROPIC_AUTH_TOKEN": "$secret:omniroute", "ANTHROPIC_API_KEY": "",
					}},
					"aider": {Command: []string{"aider", "--no-auto-commits"}, Env: map[string]string{"EDITOR": "vim"}},
				}
				if !reflect.DeepEqual(cfg.Agents, want) {
					t.Fatalf("agents %+v, want %+v", cfg.Agents, want)
				}
				return
			}
			var ce *Error
			if !errors.As(err, &ce) || ce.Line != tt.line || ce.Key != tt.key {
				t.Fatalf("error %v (%+v), want line %d key %s", err, ce, tt.line, tt.key)
			}
			if strings.Contains(err.Error(), "sk-secret") {
				t.Fatalf("the error shows the secret: %v", err)
			}
		})
	}
}

func TestAgentTitleAndSecret(t *testing.T) {
	if got := (Agent{}).TitleOf("claude-free"); got != "claude-free" {
		t.Fatalf("title %q, want the table name", got)
	}
	if got := (Agent{Title: " Free "}).TitleOf("x"); got != "Free" {
		t.Fatalf("title %q, want Free", got)
	}
	if name, ok := SecretName("$secret:omniroute"); !ok || name != "omniroute" {
		t.Fatalf("SecretName = %q, %v", name, ok)
	}
	if _, ok := SecretName("Bearer $secret:omniroute"); ok {
		t.Fatal("only a whole value names a secret")
	}
}
