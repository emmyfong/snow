package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	const rev = "759cb1324ffdd03c1aff7bcac45d9bb4c342f34d"
	vcs := func(settings ...debug.BuildSetting) *debug.BuildInfo {
		return &debug.BuildInfo{Settings: settings}
	}
	revision := debug.BuildSetting{Key: "vcs.revision", Value: rev}
	clean := debug.BuildSetting{Key: "vcs.modified", Value: "false"}
	dirty := debug.BuildSetting{Key: "vcs.modified", Value: "true"}

	tests := []struct {
		name    string
		stamped string
		info    *debug.BuildInfo
		want    string
	}{
		{"release keeps its tag", "0.1.0", vcs(revision, dirty), "0.1.0"},
		{"clean dev build", "dev", vcs(revision, clean), "dev-759cb13"},
		{"dirty dev build", "dev", vcs(revision, dirty), "dev-759cb13-dirty"},
		{"no vcs.modified", "dev", vcs(revision), "dev-759cb13"},
		{"short revision kept whole", "dev", vcs(debug.BuildSetting{Key: "vcs.revision", Value: "abc"}), "dev-abc"},
		{"no revision", "dev", vcs(dirty), "dev"},
		{"no settings", "dev", vcs(), "dev"},
		{"no build info", "dev", nil, "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildVersion(tt.stamped, tt.info); got != tt.want {
				t.Errorf("buildVersion(%q) = %q, want %q", tt.stamped, got, tt.want)
			}
		})
	}
}
