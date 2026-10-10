package main

import "runtime/debug"

// devVersion is the version a build has when no release stamped one with
// -ldflags "-X main.version=...".
const devVersion = "dev"

func init() {
	info, _ := debug.ReadBuildInfo()
	version = buildVersion(version, info)
}

// buildVersion returns stamped unchanged, unless it is still devVersion. Then
// it adds the commit from info, for example "dev-759cb13" or
// "dev-759cb13-dirty", so two development builds report different versions.
// A client compares versions with its background server, and "dev" on both
// sides hid a mismatch. Without a commit in info, it returns stamped.
func buildVersion(stamped string, info *debug.BuildInfo) string {
	if stamped != devVersion || info == nil {
		return stamped
	}
	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		return stamped
	}
	const shortHash = 7
	if len(revision) > shortHash {
		revision = revision[:shortHash]
	}
	v := stamped + "-" + revision
	if modified {
		v += "-dirty"
	}
	return v
}
