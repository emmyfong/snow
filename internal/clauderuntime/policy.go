package clauderuntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/shell"
)

// ErrPolicy means the organization's managed Claude Code policy does not
// allow a connection's route. Snow refuses; it never works around a policy
// or picks another route.
var ErrPolicy = errors.New("managed Claude Code policy blocks this route")

// Policy is what Snow can read of the managed settings files in one
// environment. Claude also reads policy from MDM, the Windows registry,
// and its server; Claude enforces those itself, and Snow shows its refusal.
type Policy struct {
	// ForceLoginMethod is "claudeai", "console", "gateway", or empty.
	ForceLoginMethod string
	// SuppliesKey is true when the policy gives Claude an API key, token,
	// or key helper, which Claude prefers over a subscription sign-in.
	SuppliesKey bool
	// Gateway is true when the policy sends Claude through a cloud gateway
	// or another address, which neither Snow route uses.
	Gateway bool
}

// policyKeyVars are managed env entries that take over authentication.
var policyKeyVars = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"}

// policyGatewayVars are managed env entries that send Claude elsewhere.
var policyGatewayVars = []string{"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY", "ANTHROPIC_BASE_URL"}

// PolicyDir is the folder of the managed settings files for a connection's
// environment: the native system folder, or /etc/claude-code inside a WSL
// distribution, which Windows reads through \\wsl.localhost.
func PolicyDir(distro string) string {
	if distro != "" {
		return shell.WSLToUNC(distro, "/etc/claude-code")
	}
	return nativePolicyDir
}

// ReadPolicy reads managed-settings.json in dir, then every *.json file
// in managed-settings.d in name order, as Claude merges them. Missing
// files are no policy. A file Snow cannot read or parse is an error, so the
// caller shows unknown rather than assuming no policy.
func ReadPolicy(dir string) (Policy, error) {
	files := []string{filepath.Join(dir, "managed-settings.json")}
	entries, err := os.ReadDir(filepath.Join(dir, "managed-settings.d"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Policy{}, fmt.Errorf("read managed settings: %w", err)
	}
	var drops []string
	for _, e := range entries {
		if name := e.Name(); !e.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".json") {
			drops = append(drops, filepath.Join(dir, "managed-settings.d", name))
		}
	}
	slices.Sort(drops)
	var p Policy
	for _, file := range append(files, drops...) {
		data, err := os.ReadFile(file)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Policy{}, fmt.Errorf("read managed settings: %w", err)
		}
		var wire struct {
			ForceLoginMethod *string        `json:"forceLoginMethod"`
			APIKeyHelper     *string        `json:"apiKeyHelper"`
			Env              map[string]any `json:"env"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return Policy{}, fmt.Errorf("read managed settings %s: %w", filepath.Base(file), err)
		}
		if wire.ForceLoginMethod != nil {
			p.ForceLoginMethod = *wire.ForceLoginMethod
		}
		if wire.APIKeyHelper != nil && *wire.APIKeyHelper != "" {
			p.SuppliesKey = true
		}
		for _, name := range policyKeyVars {
			if _, ok := wire.Env[name]; ok {
				p.SuppliesKey = true
			}
		}
		for _, name := range policyGatewayVars {
			if _, ok := wire.Env[name]; ok {
				p.Gateway = true
			}
		}
	}
	return p, nil
}

// Check returns an ErrPolicy that says why the policy does not allow auth.
func (p Policy) Check(auth AuthMode) error {
	switch {
	case p.Gateway:
		return policyError("Your organization's policy sends Claude Code through a gateway, which Snow connections do not use.")
	case auth == APIKey && p.ForceLoginMethod != "":
		return policyError("Your organization requires " + loginName(p.ForceLoginMethod) + " sign-in, and Claude Code refuses API keys.")
	case auth == Subscription && p.ForceLoginMethod != "" && p.ForceLoginMethod != "claudeai":
		return policyError("Your organization requires " + loginName(p.ForceLoginMethod) + " sign-in, not a Claude plan.")
	case auth == Subscription && p.SuppliesKey:
		return policyError("Your organization's policy supplies an API key, which Claude Code uses instead of a plan sign-in.")
	}
	return nil
}

// PolicyError is an ErrPolicy whose message is only the reason, ready to
// show the user.
type PolicyError struct{ Reason string }

// Error returns the reason.
func (e *PolicyError) Error() string { return e.Reason }

// Is makes errors.Is(err, ErrPolicy) true.
func (e *PolicyError) Is(target error) bool { return target == ErrPolicy }

func policyError(reason string) error { return &PolicyError{Reason: reason} }

func loginName(method string) string {
	switch method {
	case "claudeai":
		return "Claude subscription"
	case "console":
		return "Claude Console"
	case "gateway":
		return "gateway"
	}
	return "another"
}
