# kdbx-secrets bootstrap (Windows).
#   irm https://raw.githubusercontent.com/costaluu/kdbx-secrets/main/install.ps1 | iex
# Variables: KDBX_SECRETS_BIN_DIR (destination), KDBX_SECRETS_BASE_URL (tests only).
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = 'costaluu/kdbx-secrets'
$base = if ($env:KDBX_SECRETS_BASE_URL) { $env:KDBX_SECRETS_BASE_URL } else { "https://github.com/$repo/releases/latest/download" }
$binDir = if ($env:KDBX_SECRETS_BIN_DIR) { $env:KDBX_SECRETS_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'kdbx-secrets\bin' }

if (-not [Environment]::Is64BitOperatingSystem -or $env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
    throw 'unsupported architecture (windows/amd64 only)'
}
$asset = 'kdbx-secrets-windows-amd64.exe'

$tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Invoke-WebRequest -UseBasicParsing "$base/$asset" -OutFile (Join-Path $tmp $asset)
    Invoke-WebRequest -UseBasicParsing "$base/SHA256SUMS" -OutFile (Join-Path $tmp 'SHA256SUMS')

    $line = Get-Content (Join-Path $tmp 'SHA256SUMS') | Where-Object { ($_ -split '\s+', 2)[1] -in @($asset, "*$asset") } | Select-Object -First 1
    if (-not $line) { throw "$asset is not listed in SHA256SUMS" }
    $want = ($line -split '\s+')[0].ToLower()
    $got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $asset)).Hash.ToLower()
    if ($got -ne $want) { throw "SHA-256 mismatch (expected $want, got $got); nothing was installed" }
    Write-Host 'SHA-256 ok'

    if (Get-Command gh -ErrorAction SilentlyContinue) {
        & gh attestation verify (Join-Path $tmp $asset) --repo $repo
        if ($LASTEXITCODE -ne 0) { throw 'invalid attestation; nothing was installed' }
    } else {
        Write-Warning "gh not found; to verify provenance by hand:`n  gh attestation verify $asset --repo $repo"
    }

    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    $destTmp = Join-Path $binDir ".kdbx-secrets.$PID.tmp"
    Copy-Item (Join-Path $tmp $asset) $destTmp -Force
    Move-Item -Force $destTmp (Join-Path $binDir 'kdbx-secrets.exe')   # rename on the same volume
    Write-Host "installed to $binDir\kdbx-secrets.exe"

    $inPath = ($env:PATH -split ';') | Where-Object { $_.TrimEnd('\') -ieq $binDir.TrimEnd('\') }
    if (-not $inPath) { Write-Warning "$binDir is not in PATH" }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
