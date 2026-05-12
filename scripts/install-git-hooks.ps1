param(
    [switch]$Force,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Usage {
    Write-Host @"
Install local Git hooks for this repository.

Usage:
  .\scripts\install-git-hooks.ps1
  .\scripts\install-git-hooks.ps1 -Force

The installed pre-commit hook runs:
  .\scripts\pre-commit.ps1
"@
}

function Fail {
    param([string]$Message)

    Write-Error $Message
    exit 1
}

if ($Help) {
    Show-Usage
    exit 0
}

$repoRoot = git rev-parse --show-toplevel
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($repoRoot)) {
    Fail "not inside a git repository"
}

$repoRoot = $repoRoot.Trim()
$hookDir = Join-Path $repoRoot ".git\hooks"
$hookPath = Join-Path $hookDir "pre-commit"

if ((Test-Path $hookPath) -and -not $Force) {
    Fail "pre-commit hook already exists; rerun with -Force to replace it"
}

if (-not (Test-Path $hookDir)) {
    New-Item -ItemType Directory -Force -Path $hookDir | Out-Null
}

$hook = @'
#!/bin/sh
set -eu

repo_root=$(git rev-parse --show-toplevel)
script="$repo_root/scripts/pre-commit.ps1"

if command -v pwsh >/dev/null 2>&1; then
  exec pwsh -NoProfile -ExecutionPolicy Bypass -File "$script"
fi

if command -v powershell.exe >/dev/null 2>&1; then
  exec powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$script"
fi

echo "PowerShell is required to run scripts/pre-commit.ps1" >&2
exit 1
'@

$hook = $hook.Replace("`r`n", "`n")
$utf8NoBom = [System.Text.UTF8Encoding]::new($false)
[System.IO.File]::WriteAllText($hookPath, $hook + "`n", $utf8NoBom)

$chmod = Get-Command chmod -ErrorAction SilentlyContinue
if ($chmod) {
    & chmod +x $hookPath
}

Write-Host "installed pre-commit hook: $hookPath"
