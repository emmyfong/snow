package server

import (
	"slices"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/gittest"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

const sidePaneAgentProfile = "Claude Code (Ubuntu)"

func TestSidePaneTerminalIsShell(t *testing.T) {
	s, c, _, loaded := sidePaneFromAgent(t)
	c.Send(&api.Command{Op: api.OpNewWindow, Args: map[string]string{api.ArgKind: api.PaneTerminal}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 2 && l.Active == 1 })
	panes := shownPanes(l)
	if len(panes) != 1 || panes[0].Profile != "Ubuntu" {
		t.Fatalf("new window panes %+v, want the Ubuntu host shell", panes)
	}
	if distro, ok := s.launch.distro(panes[0].Profile); !ok || distro != "Ubuntu" {
		t.Fatalf("new window distribution %q, %v; want Ubuntu", distro, ok)
	}
	select {
	case names := <-loaded:
		t.Fatalf("new host-shell window loaded secrets %v, want none", names)
	default:
	}
}

func TestSidePaneTerminalHasNoSecrets(t *testing.T) {
	s, _, _, _ := sidePaneFromAgent(t)
	cmd := &api.Command{Op: api.OpNewWindow, Args: map[string]string{api.ArgKind: api.PaneTerminal}}
	var got []string
	s.call(func(st *state) { got = st.paneSecrets(st.sessions["work"], cmd) })
	if len(got) != 0 {
		t.Fatalf("new terminal secrets %v, want no Claude Code secrets for the host shell", got)
	}
}

func TestSidePaneLabelIsKind(t *testing.T) {
	_, _, gitPane, _ := sidePaneFromAgent(t)
	if gitPane.Profile != "" || gitPane.Kind != api.PaneGit {
		t.Fatalf("side pane %+v, want empty profile and kind %q", gitPane, api.PaneGit)
	}
}

func sidePaneFromAgent(t *testing.T) (*Server, *apitest.Client, api.PaneInfo, chan []string) {
	return sidePaneFromAgentWithDistro(t, true)
}

func sidePaneFromNativeAgent(t *testing.T) (*Server, *apitest.Client, api.PaneInfo, chan []string) {
	return sidePaneFromAgentWithDistro(t, false)
}

func sidePaneFromAgentWithDistro(t *testing.T, wsl bool) (*Server, *apitest.Client, api.PaneInfo, chan []string) {
	t.Helper()
	dir := t.TempDir()
	gittest.Isolate(t)
	gittest.Repo(t, dir)
	opts := claudeOptions(nil, nil)
	opts.agentProfile = func(profile string) bool { return strings.HasPrefix(profile, "Claude Code") }
	opts.afterHookWait = func(func()) {}
	host, agentProfile := hostShell(), "Claude Code"
	if wsl {
		host, agentProfile = shell.Profile{Name: "Ubuntu", Kind: shell.KindWSL, Distro: "Ubuntu"}, sidePaneAgentProfile
	}
	opts.Profiles = func() []string { return []string{host.Name, agentProfile} }
	spec := opts.Spec
	opts.Spec = func(profile, dir string) (term.Spec, string, error) {
		if profile == "" {
			profile = host.Name
		}
		return spec(profile, dir)
	}
	s := New(opts)
	t.Cleanup(s.Close)
	s.launch.shell = func(profile string) (shell.Profile, bool) {
		if strings.HasPrefix(profile, "Claude Code") || profile == host.Name || profile == "" {
			return host, true
		}
		return shell.Profile{}, false
	}
	if wsl {
		s.launch.distro = func(profile string) (string, bool) {
			return host.Distro, profile == host.Name
		}
	}
	loaded := make(chan []string, 4)
	s.launch.secrets = func(profile string) []string {
		switch profile {
		case agentProfile:
			return []string{"agent-secret"}
		default:
			return nil
		}
	}
	s.launch.loadSecrets = func(names []string) error {
		loaded <- slices.Clone(names)
		return nil
	}
	c := connect(t, s, 120, 30)
	first := attachIn(t, c, dir)
	c.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{
		api.ArgDir: api.DirRight, api.ArgProfile: agentProfile,
	}})
	if names := <-loaded; !slices.Equal(names, []string{"agent-secret"}) {
		t.Fatalf("agent launch loaded secrets %v, want Claude Code secrets", names)
	}
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	var agent int
	for _, p := range shownPanes(l) {
		if p.Profile == agentProfile {
			agent = p.ID
		}
	}
	if agent == 0 {
		t.Fatalf("no Claude Code pane in %+v", shownPanes(l))
	}
	c.Send(&api.Command{Op: api.OpOpen, Target: agent, Args: map[string]string{api.ArgKind: api.PaneGit}})
	l = nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 3 })
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneGit {
			return s, c, p, loaded
		}
	}
	t.Fatal("no git pane opened from the Claude Code pane")
	return nil, nil, api.PaneInfo{}, nil
}
