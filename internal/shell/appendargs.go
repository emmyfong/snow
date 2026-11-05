package shell

import (
	"slices"
	"strings"
)

// AppendArgs returns a launch's arguments and Windows command line with
// extra added after the arguments it has. A launch that runs a .cmd or
// .bat script through cmd (see Program) ends in the quoted script line, so
// extra goes inside it, each argument quoted for cmd; an argument cmd
// cannot pass safely gives ErrScriptArg. args and cmdLine are not changed.
func AppendArgs(args []string, cmdLine string, extra ...string) ([]string, string, error) {
	if cmdLine == "" || len(extra) == 0 {
		return append(slices.Clone(args), extra...), cmdLine, nil
	}
	var quoted strings.Builder
	for _, a := range extra {
		q, err := cmdArg(a)
		if err != nil {
			return nil, "", err
		}
		quoted.WriteString(" " + q)
	}
	if n := len(args); n >= 4 && args[n-2] == "/C" && strings.HasSuffix(cmdLine, `"`) {
		out := slices.Clone(args)
		out[n-1] += quoted.String()
		return out, cmdLine[:len(cmdLine)-1] + quoted.String() + `"`, nil
	}
	return append(slices.Clone(args), extra...), cmdLine + quoted.String(), nil
}
