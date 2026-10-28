//go:build windows || linux

package notify

import (
	"context"
	"os"
	"os/exec"

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
	cmd.Env = append(os.Environ(), "SNOW_NOTIFY_TITLE="+title, "SNOW_NOTIFY_BODY="+body)
	shell.HideWindow(cmd)
	return cmd
}
