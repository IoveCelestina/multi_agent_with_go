param(
    [switch]$AllowMain,
    [switch]$SkipTests,
    [switch]$SkipVet,
    [switch]$Fix,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Usage {
    Write-Host @"
Run local checks before committing.

Usage:
  .\scripts\pre-commit.ps1
  .\scripts\pre-commit.ps1 -AllowMain
  .\scripts\pre-commit.ps1 -Fix

Checks:
  - current branch follows docs/GIT_WORKFLOW.md
  - staged secret-like files are blocked
  - staged Go filenames use snake_case
  - Go files are formatted with gofmt
  - go vet ./...
  - go test ./...

Environment:
  ALLOW_MAIN_COMMIT=1 allows commits on main.
  SKIP_GO_TESTS=1 skips go test ./...
  SKIP_GO_VET=1 skips go vet ./...
"@
}

function Write-Step {
    param([string]$Message)

    Write-Host "==> $Message"
}

function Fail {
    param([string]$Message)

    Write-Error $Message
    exit 1
}

function Invoke-Git {
    param([string[]]$Arguments)

    $output = & git @Arguments
    if ($LASTEXITCODE -ne 0) {
        Fail "git $($Arguments -join ' ') failed"
    }

    return $output
}

function Test-BlockedPath {
    param([string]$Path)

    $normalized = $Path -replace "\\", "/"
    $leaf = Split-Path -Leaf $normalized

    if ($leaf -eq ".env" -or $leaf -like ".env.*") {
        return $true
    }

    if ($normalized -match "(^|/)(id_rsa|id_dsa|id_ecdsa|id_ed25519)$") {
        return $true
    }

    if ($normalized -match "\.(pem|key|p12|pfx)$") {
        return $true
    }

    return $false
}

function Test-GoFileName {
    param([string]$Path)

    $leaf = Split-Path -Leaf $Path
    return $leaf -match "^[a-z0-9]+(_[a-z0-9]+)*(_test)?\.go$"
}

if ($Help) {
    Show-Usage
    exit 0
}

$repoRoot = @(Invoke-Git -Arguments @("rev-parse", "--show-toplevel"))[0]
Set-Location $repoRoot

Write-Step "checking git branch"
$branch = @(Invoke-Git -Arguments @("branch", "--show-current"))[0]

if ([string]::IsNullOrWhiteSpace($branch)) {
    Write-Host "Detached HEAD; skipping branch name check."
} elseif ($branch -eq "main") {
    if (-not $AllowMain -and $env:ALLOW_MAIN_COMMIT -ne "1") {
        Fail "committing directly to main is blocked; create a review branch with .\scripts\new-branch.ps1 or rerun with -AllowMain"
    }
} elseif ($branch -notmatch "^ht[0-9]([1-9]|1[0-2])([1-9]|[a-v])-dev[1-9][0-9]*$") {
    Fail "invalid branch name '$branch'; expected ht<year_last><month><day_code>-dev<n>"
}

Write-Step "checking staged files"
$stagedFiles = @(git diff --cached --name-only --diff-filter=ACMR)
if ($LASTEXITCODE -ne 0) {
    Fail "failed to list staged files"
}

foreach ($file in $stagedFiles) {
    if (Test-BlockedPath $file) {
        Fail "blocked staged file '$file'; do not commit env files, private keys, or certificate bundles"
    }

    if ($file -like "*.go" -and -not (Test-GoFileName $file)) {
        Fail "Go filename '$file' must use snake_case, for example tool_executor.go"
    }
}

if (-not $env:GOCACHE) {
    $env:GOCACHE = Join-Path $repoRoot ".gocache"
}

if (-not (Test-Path $env:GOCACHE)) {
    New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
}

$goFiles = @(git ls-files "*.go")
if ($LASTEXITCODE -ne 0) {
    Fail "failed to list Go files"
}

if ($goFiles.Count -gt 0) {
    Write-Step "checking gofmt"
    if ($Fix) {
        & gofmt -w @goFiles
        if ($LASTEXITCODE -ne 0) {
            Fail "gofmt -w failed"
        }
    } else {
        $unformatted = @(gofmt -l @goFiles)
        if ($LASTEXITCODE -ne 0) {
            Fail "gofmt -l failed"
        }

        if ($unformatted.Count -gt 0) {
            Write-Host "Unformatted files:"
            $unformatted | ForEach-Object { Write-Host "  $_" }
            Fail "run gofmt on the files above or rerun .\scripts\pre-commit.ps1 -Fix"
        }
    }

    if (-not $SkipVet -and $env:SKIP_GO_VET -ne "1") {
        Write-Step "running go vet ./..."
        & go vet ./...
        if ($LASTEXITCODE -ne 0) {
            Fail "go vet ./... failed"
        }
    }

    if (-not $SkipTests -and $env:SKIP_GO_TESTS -ne "1") {
        Write-Step "running go test ./..."
        & go test ./...
        if ($LASTEXITCODE -ne 0) {
            Fail "go test ./... failed"
        }
    }
}

Write-Host "pre-commit checks passed"
