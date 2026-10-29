//go:build windows || linux

package notify

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/emmyfong/snow/internal/shell"
)

// toastScript reads the text from the environment, so no session name or
// other text is ever parsed as PowerShell code.
const toastScript = `$ErrorActionPreference = 'Stop'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
$t = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$text = $t.GetElementsByTagName('text')
$text.Item(0).AppendChild($t.CreateTextNode($env:SNOW_NOTIFY_TITLE)) | Out-Null
$text.Item(1).AppendChild($t.CreateTextNode($env:SNOW_NOTIFY_BODY)) | Out-Null
$toast = [Windows.UI.Notifications.ToastNotification]::new($t)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($toast)`

// toastCommand returns the PowerShell command that shows n as a Windows
// toast. powershell is the program to run: powershell.exe on Windows, and
// the Windows interop program in WSL.
func toastCommand(ctx context.Context, n Notice, powershell string) *exec.Cmd {
	title, body := message(n)
	cmd := exec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", toastScript)
	env := append(os.Environ(), "SNOW_NOTIFY_TITLE="+title, "SNOW_NOTIFY_BODY="+body)
	// WSL passes only the variables WSLENV names to a Windows program. The
	// script reads the text from the environment, so both names must be
	// listed there; an existing WSLENV stays, with its other entries.
	cmd.Env = withWSLEnv(env, "SNOW_NOTIFY_TITLE", "SNOW_NOTIFY_BODY")
	shell.HideWindow(cmd)
	return cmd
}

// withWSLEnv returns env with names added to WSLENV, so WSL passes them to
// the Windows program. Other entries keep their order.
func withWSLEnv(env []string, names ...string) []string {
	var value string
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "WSLENV="); ok {
			value = v
			continue
		}
		out = append(out, e)
	}
	var entries []string
	if value != "" {
		entries = strings.Split(value, ":")
	}
	for _, name := range names {
		if !hasWSLEntry(entries, name) {
			entries = append(entries, name)
		}
	}
	return append(out, "WSLENV="+strings.Join(entries, ":"))
}

// hasWSLEntry reports whether entries names entry, with or without WSLENV
// flags such as /p.
func hasWSLEntry(entries []string, entry string) bool {
	for _, e := range entries {
		if e == entry || strings.HasPrefix(e, entry+"/") {
			return true
		}
	}
	return false
}
