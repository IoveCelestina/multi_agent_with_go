# Git Workflow

This project uses short-lived review branches. Changes should be made on a review branch, checked, committed, reviewed, and then merged back to `main`.

## Branch Naming

Branch names use this format:

```text
ht<year_last><month><day_code>-dev<n>
```

Parts:

- `ht`: fixed prefix.
- `year_last`: the last digit of the year.
- `month`: month number, without leading zero.
- `day_code`: day of month. Use digits `1`-`9` for days 1-9, and letters `a`-`v` for days 10-31.
- `dev<n>`: development sequence for that day, starting at `dev1`.

Day code mapping:

```text
1  -> 1
2  -> 2
3  -> 3
4  -> 4
5  -> 5
6  -> 6
7  -> 7
8  -> 8
9  -> 9
10 -> a
11 -> b
12 -> c
13 -> d
14 -> e
15 -> f
16 -> g
17 -> h
18 -> i
19 -> j
20 -> k
21 -> l
22 -> m
23 -> n
24 -> o
25 -> p
26 -> q
27 -> r
28 -> s
29 -> t
30 -> u
31 -> v
```

Examples:

```text
2026-05-11 dev1 -> ht65b-dev1
2026-05-11 dev2 -> ht65b-dev2
2026-05-09 dev1 -> ht659-dev1
2026-12-31 dev1 -> ht612v-dev1
```

## Creating A Branch

Use the helper script from `main`:

```powershell
.\scripts\new-branch.ps1
```

Create the second branch for the same day:

```powershell
.\scripts\new-branch.ps1 -Dev 2
```

Create an explicit branch:

```powershell
.\scripts\new-branch.ps1 ht65b-dev3
```

The script refuses to create a branch from a dirty working tree unless `-AllowDirty` is passed.

## Merge Rule

Do not commit directly to `main` for normal changes.

Workflow:

```text
main -> review branch -> commit -> review -> merge to main
```

Use fast-forward merge when possible:

```powershell
git switch main
git merge --ff-only <branch-name>
```

## Pre-Commit Checks

Install the local Git hook once per clone:

```powershell
.\scripts\install-git-hooks.ps1
```

Run the same checks manually:

```powershell
.\scripts\pre-commit.ps1
```

The pre-commit check enforces the branch naming rule, blocks common secret files,
checks Go filenames, verifies `gofmt`, and runs `go vet ./...` and `go test ./...`.

For exceptional repository maintenance commits on `main`, run the script with:

```powershell
.\scripts\pre-commit.ps1 -AllowMain
```
