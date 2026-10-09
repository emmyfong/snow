# Snow folder hook for PowerShell. Snow runs this with -NoExit -Command after
# the user's profile has loaded. It wraps the existing prompt function and
# reports the current folder with OSC 7. Snow never edits profiles.
$global:__SnowPrompt = $function:prompt
function global:prompt {
    $loc = $executionContext.SessionState.Path.CurrentLocation
    if ($loc.Provider.Name -eq 'FileSystem') {
        $path = $loc.ProviderPath -replace '\\', '/'
        $esc = [char]27
        [Console]::Write("$esc]7;file://$env:COMPUTERNAME/$path$esc\")
    }
    & $global:__SnowPrompt
}
