# Install Snow on Windows:
#
#   irm https://raw.githubusercontent.com/emmyfong/snow/main/install.ps1 | iex
#
# It downloads the newest release for this machine, checks it against the
# release's checksums, installs snow.exe to %LOCALAPPDATA%\Programs\snow, and
# adds that folder to your user PATH. Run it again to upgrade, even while
# Snow is running. Settings:
#
#   $env:SNOW_VERSION      a tag such as v0.1.0; default: the newest release
#   $env:SNOW_INSTALL_DIR  where snow.exe goes
#   $env:SNOW_BASE_URL     where the release files are: a URL or a folder;
#                          for tests and mirrors
#   $env:SNOW_NO_PATH      set to 1 to leave PATH alone
#
# It runs inside your PowerShell session (irm | iex), so it never calls exit:
# a failure stops the script, not your terminal.

function Install-Snow {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # the progress bar slows downloads in PowerShell 5.1

    # Windows PowerShell 5.1 may default to TLS 1.0, which GitHub refuses.
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $arch = Get-SnowArch
    if ($env:SNOW_BASE_URL) {
        $base = $env:SNOW_BASE_URL
    } elseif ($env:SNOW_VERSION) {
        $base = "https://github.com/emmyfong/snow/releases/download/$($env:SNOW_VERSION)"
    } else {
        $base = 'https://github.com/emmyfong/snow/releases/latest/download'
    }
    $dest = if ($env:SNOW_INSTALL_DIR) { $env:SNOW_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\snow' }
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    # A relative folder would go into PATH relative to wherever a shell runs.
    $dest = [IO.Path]::GetFullPath($dest)
    $archive = "snow_windows_$arch.zip"

    # The temp folder sits beside the destination, so the final move stays
    # on one drive and an interrupted install never leaves half a binary.
    $tmp = Join-Path $dest (".snow-install-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    $target = Join-Path $dest 'snow.exe'
    try {
        Write-Host "Downloading $archive"
        Get-SnowFile "$base/$archive" (Join-Path $tmp $archive)
        Get-SnowFile "$base/checksums.txt" (Join-Path $tmp 'checksums.txt')

        $want = $null
        foreach ($line in Get-Content -LiteralPath (Join-Path $tmp 'checksums.txt')) {
            $fields = $line -split '\s+'
            if ($fields.Count -ge 2 -and $fields[1] -eq $archive) { $want = $fields[0] }
        }
        if (-not $want) { throw "snow install: checksums.txt has no entry for $archive" }
        $got = Get-SnowSha256 (Join-Path $tmp $archive)
        if ($got -ne $want) {
            throw "snow install: checksum mismatch for ${archive}: the download is damaged or was changed; nothing was installed"
        }

        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [IO.Compression.ZipFile]::ExtractToDirectory((Join-Path $tmp $archive), (Join-Path $tmp 'unpacked'))
        $exe = Join-Path $tmp 'unpacked\snow.exe'
        if (-not (Test-Path -LiteralPath $exe)) { throw 'snow install: the archive has no snow.exe' }
        Replace-SnowExe $exe $target
    } finally {
        Remove-Item -Recurse -Force -LiteralPath $tmp -ErrorAction SilentlyContinue
    }

    $version = & $target version
    Write-Host "Installed $version to $target"
    if ($env:SNOW_NO_PATH -ne '1') { Add-SnowPath $dest }
}

# Get-SnowSha256 hashes a file with .NET, not Get-FileHash. Windows
# PowerShell started from PowerShell 7 inherits its PSModulePath, loads the
# wrong Microsoft.PowerShell.Utility, and has no Get-FileHash; Expand-Archive
# fails the same way, so the archive is unpacked with .NET too.
function Get-SnowSha256($path) {
    $sha = [Security.Cryptography.SHA256]::Create()
    $stream = [IO.File]::OpenRead($path)
    try {
        return [BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-', '')
    } finally {
        $stream.Dispose()
        $sha.Dispose()
    }
}

# Get-SnowArch returns amd64 or arm64 for the machine, not for this
# PowerShell: an x64 PowerShell on an ARM64 PC runs emulated and reports
# AMD64, and a 32-bit one on 64-bit Windows reports x86.
function Get-SnowArch {
    $cpu = $null
    try { $cpu = (Get-CimInstance -ClassName Win32_Processor -ErrorAction Stop | Select-Object -First 1).Architecture } catch { }
    switch ($cpu) {
        9 { return 'amd64' }
        12 { return 'arm64' }
    }
    $machine = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    switch ($machine) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
    }
    throw "snow install: no release for $machine"
}

# Replace-SnowExe moves the new snow.exe into place. Windows refuses to
# overwrite a running exe but lets it be renamed, and Snow's server keeps
# running between sessions, so the old file moves aside first. A newer snow
# then offers to restart the old server.
function Replace-SnowExe($new, $target) {
    $old = "$target.old"
    Remove-Item -Force -LiteralPath $old -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $target) {
        try {
            Move-Item -Force -LiteralPath $target -Destination $old
        } catch {
            throw "snow install: could not move the old $target aside: $($_.Exception.Message)"
        }
    }
    try {
        Move-Item -Force -LiteralPath $new -Destination $target
    } catch {
        if (Test-Path -LiteralPath $old) { Move-Item -Force -LiteralPath $old -Destination $target -ErrorAction SilentlyContinue }
        throw "snow install: could not install ${target}: $($_.Exception.Message)"
    }
    # Still running: the next install removes it.
    Remove-Item -Force -LiteralPath $old -ErrorAction SilentlyContinue
}

# Add-SnowPath adds dir to the user PATH. It edits the raw registry value:
# [Environment]::SetEnvironmentVariable would expand entries such as
# %USERPROFILE%\bin into fixed paths and store the value as a plain string.
# SNOW_PATH_KEY, for tests, names another key under HKCU instead.
function Add-SnowPath($dir) {
    $keyName = if ($env:SNOW_PATH_KEY) { $env:SNOW_PATH_KEY } else { 'Environment' }
    $key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($keyName)
    try {
        $raw = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $parts = @($raw -split ';' | Where-Object { $_ })
        $known = $parts | ForEach-Object { [Environment]::ExpandEnvironmentVariables($_).TrimEnd('\') }
        if ($known -notcontains $dir.TrimEnd('\')) {
            $key.SetValue('Path', (($parts + $dir) -join ';'), [Microsoft.Win32.RegistryValueKind]::ExpandString)
            if (-not $env:SNOW_PATH_KEY) {
                # Setting a user variable tells running programs, such as
                # Explorer, that the environment changed.
                [Environment]::SetEnvironmentVariable('SNOW_INSTALL_REFRESH', '1', 'User')
                [Environment]::SetEnvironmentVariable('SNOW_INSTALL_REFRESH', $null, 'User')
            }
            Write-Host "Added $dir to your user PATH. Terminals opened before now need a restart to see it."
        }
    } finally {
        $key.Close()
    }
    if (($env:Path -split ';') -notcontains $dir) { $env:Path = "$env:Path;$dir" }
}

# Get-SnowFile copies one release file. HTTPS only, so a URL cannot point at
# plain http; a local folder serves tests and mirrors.
function Get-SnowFile($from, $to) {
    if ($from -match '^https://') {
        try {
            Invoke-WebRequest -UseBasicParsing -Uri $from -OutFile $to
        } catch {
            throw "snow install: could not download ${from}: $($_.Exception.Message)"
        }
    } elseif ($from -match '^[a-z]+://') {
        throw "snow install: only https:// URLs are allowed, not $from"
    } else {
        Copy-Item -LiteralPath $from -Destination $to
    }
}

try {
    Install-Snow
} catch {
    # Rethrown as a new error: run with -File, some failures, such as a
    # missing command, would otherwise stop the script with exit code 0.
    $message = $_.Exception.Message
    if ($message -notlike 'snow install:*') { $message = "snow install: $message" }
    throw $message
} finally {
    # irm | iex defines these in the user's session; leave nothing behind.
    Remove-Item -ErrorAction SilentlyContinue -Path Function:\Install-Snow, Function:\Get-SnowSha256, Function:\Get-SnowArch, Function:\Replace-SnowExe, Function:\Add-SnowPath, Function:\Get-SnowFile
}
