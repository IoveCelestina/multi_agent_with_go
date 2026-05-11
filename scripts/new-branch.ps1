param(
    [Parameter(Position = 0)]
    [string]$Name,

    [string]$Base = "main",

    [switch]$AllowDirty,

    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Usage {
    Write-Host @"
Create a review branch from a base branch.

Usage:
  .\scripts\new-branch.ps1 <type/name> [-Base main] [-AllowDirty]

Examples:
  .\scripts\new-branch.ps1 feat/provider-streaming
  .\scripts\new-branch.ps1 docs/coding-style
  .\scripts\new-branch.ps1 chore/update-scripts -AllowDirty

Branch naming:
  feat/<name>
  fix/<name>
  docs/<name>
  chore/<name>
  test/<name>
  refactor/<name>
"@
}

if ($Help) {
    Show-Usage
    exit 0
}

if ([string]::IsNullOrWhiteSpace($Name)) {
    Show-Usage
    throw "branch name is required"
}

if ($Name -notmatch "^(feat|fix|docs|chore|test|refactor)/[a-z0-9][a-z0-9._-]*$") {
    throw "invalid branch name: $Name"
}

$insideWorkTree = git rev-parse --is-inside-work-tree 2>$null
if ($LASTEXITCODE -ne 0 -or $insideWorkTree -ne "true") {
    throw "not inside a git repository"
}

$dirty = git status --porcelain
if ($dirty -and -not $AllowDirty) {
    throw "working tree is not clean; commit/stash changes or rerun with -AllowDirty"
}

git show-ref --verify --quiet "refs/heads/$Name"
if ($LASTEXITCODE -eq 0) {
    throw "branch already exists: $Name"
}

git show-ref --verify --quiet "refs/heads/$Base"
if ($LASTEXITCODE -ne 0) {
    throw "base branch does not exist: $Base"
}

git switch $Base
if ($LASTEXITCODE -ne 0) {
    throw "failed to switch to base branch: $Base"
}

git switch -c $Name
if ($LASTEXITCODE -ne 0) {
    throw "failed to create branch: $Name"
}

Write-Host "created and switched to branch: $Name"
