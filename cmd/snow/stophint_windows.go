package main

// stopHint is the PowerShell command that stops a server too old to restart
// itself. It ends only snow.exe processes started as the server, not other
// snow windows.
const stopHint = `Get-CimInstance Win32_Process -Filter "Name='snow.exe'" | Where-Object CommandLine -match ' server$' | ForEach-Object { Stop-Process -Id $_.ProcessId }`
