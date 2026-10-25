// Package tool holds the tools an agent can call and the registry that
// names them. Each tool lives in its own file and registers itself in the
// built-in registry, so a new tool changes no other file.
package tool

import (
	"context"
	"encoding/json"

	"github.com/emmyfong/snow/internal/shell"
)

// Def describes a tool to the model and to the approval step.
type Def struct {
	Name        string
	Description string
	Schema      json.RawMessage // a JSON Schema object for the input
	// Changes is true for a tool that changes files or runs commands. In
	// the ask mode such a tool waits for the user's approval.
	Changes bool
}

// Tool is one tool an agent can call.
type Tool interface {
	Def() Def
	// Run runs the tool with the model's input and returns the text the
	// model gets back. An error also goes back to the model, as a failed
	// result; it never stops the turn. Run must stop when ctx ends.
	Run(ctx context.Context, env Env, input json.RawMessage) (string, error)
}

// Env is what a tool may use beyond its input.
type Env struct {
	// Folder is the conversation folder. Tools resolve relative paths
	// against it.
	Folder Folder
	// Shell is the profile saved with the conversation. The run tool runs
	// commands through it.
	Shell shell.Profile
	// HideEnv names environment variables that commands do not get, such
	// as the variables that hold provider keys.
	HideEnv []string
}
