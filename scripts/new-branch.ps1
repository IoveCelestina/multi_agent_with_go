param(
    [Parameter(Position = 0)]
    [string]$Name,

    [string]$Base = "main",

    [int]$Dev = 1,

    [switch]$AllowDirty,

    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Usage {
    Write-Host @"
Create a review branch from a base branch.

Usage:
  .\scripts\new-branch.ps1 [-Dev 1] [-Base main] [-AllowDirty]
  .\scripts\new-branch.ps1 <branch-name> [-Base main] [-AllowDirty]

Examples:
  .\scripts\new-branch.ps1
  .\scripts\new-branch.ps1 -Dev 2
  .\scripts\new-branch.ps1 ht65b-dev3

Branch naming:
  ht<year_last><month><day_code>-dev<n>

Day code:
  1-9 use digits 1-9
  10-31 use letters a-v

Example:
  2026-05-11 dev1 => ht65b-dev1
"@
}

function Get-DayCode {
    param([int]$Day)

    if ($Day -lt 1 -or $Day -gt 31) {
        throw "invalid day: $Day"
    }

    if ($Day -lt 10) {
        return [string]$Day
    }

    $offset = $Day - 10
    return [string][char]([int][char]"a" + $offset)
}

function New-BranchName {
    param(
        [datetime]$Date,
        [int]$DevNumber
    )

    if ($DevNumber -lt 1) {
        throw "dev number must be greater than zero"
    }

    $yearLast = $Date.Year % 10
    $month = $Date.Month
    $dayCode = Get-DayCode -Day $Date.Day

    return "ht$yearLast$month$dayCode-dev$DevNumber"
}

if ($Help) {
    Show-Usage
    exit 0
}

if ([string]::IsNullOrWhiteSpace($Name)) {
    $Name = New-BranchName -Date (Get-Date) -DevNumber $Dev
}

if ($Name -notmatch "^ht[0-9]([1-9]|1[0-2])([1-9]|[a-v])-dev[1-9][0-9]*$") {
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
