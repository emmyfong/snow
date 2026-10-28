package main

import (
	"errors"
	"fmt"
	"strings"
)

// kind is what a command line asks snow to do.
type kind int

const (
	kindAttach kind = iota
	kindServer
	kindVersion
	kindHelp
	kindAuth
	kindAuthHelp
	kindHook
	kindClaude
	kindClaudeHelp
	kindMCP
)

// command is a parsed command line.
type command struct {
	kind    kind
	session string // for kindAttach; "" creates a numbered session
	auth    authCommand
	claude  claudeAction
}

func parseArgs(args []string) (command, error) {
	switch {
	case len(args) == 0:
		return command{kind: kindAttach}, nil
	case args[0] == "auth":
		if len(args) == 2 && (args[1] == "help" || args[1] == "-h" || args[1] == "--help") {
			return command{kind: kindAuthHelp}, nil
		}
		a, err := parseAuth(args[1:])
		if err != nil {
			return command{kind: kindAuthHelp}, err
		}
		return command{kind: kindAuth, auth: a}, nil
	case args[0] == "claude":
		return parseClaudeCommand(args[1:])
	case len(args) > 1:
		return command{}, errors.New("too many arguments")
	}
	switch a := args[0]; a {
	case "version", "-v", "--version":
		return command{kind: kindVersion}, nil
	case "help", "-h", "--help":
		return command{kind: kindHelp}, nil
	case "server":
		return command{kind: kindServer}, nil
	case "hook":
		return command{kind: kindHook}, nil
	case "mcp":
		return command{kind: kindMCP}, nil
	default:
		if strings.HasPrefix(a, "-") {
			return command{}, fmt.Errorf("unknown option %s", a)
		}
		if strings.ContainsAny(a, `/\`) {
			return command{}, fmt.Errorf("session names cannot contain / or \\: %s", a)
		}
		return command{kind: kindAttach, session: a}, nil
	}
}
