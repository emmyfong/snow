package shell

import (
	"path"
	"strings"
)

// uncPrefix starts every path the Windows side uses to reach a WSL
// distribution's files.
const uncPrefix = `\\wsl.localhost\`

// WSLToUNC returns the Windows path of a Linux path in a WSL distribution,
// for example \\wsl.localhost\Ubuntu\home\u for /home/u.
func WSLToUNC(distro, linux string) string {
	return uncPrefix + distro + strings.ReplaceAll(path.Clean("/"+linux), "/", `\`)
}

// UNCToWSL is the reverse of WSLToUNC. It also takes the older \\wsl$
// form. ok is false for a path that is not in a WSL distribution.
func UNCToWSL(unc string) (distro, linux string, ok bool) {
	rest, found := cutPrefixFold(unc, uncPrefix)
	if !found {
		if rest, found = cutPrefixFold(unc, `\\wsl$\`); !found {
			return "", "", false
		}
	}
	distro, tail, _ := strings.Cut(rest, `\`)
	if distro == "" {
		return "", "", false
	}
	return distro, path.Clean("/" + strings.ReplaceAll(tail, `\`, "/")), true
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return s, false
	}
	return s[len(prefix):], true
}
