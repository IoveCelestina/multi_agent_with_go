# Git 工作流

本项目使用短生命周期 review 分支。常规改动应在 review 分支上完成，检查、提交、审查后再合并回 `main`。

## 分支命名

分支名格式：

```text
ht<year_last><month><day_code>-dev<n>
```

各部分含义：

- `ht`：固定前缀。
- `year_last`：年份最后一位。
- `month`：月份数字，不补零。
- `day_code`：日期编码。1-9 日使用数字 `1`-`9`，10-31 日使用字母 `a`-`v`。
- `dev<n>`：当天开发序号，从 `dev1` 开始。

日期编码：

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

示例：

```text
2026-05-11 dev1 -> ht65b-dev1
2026-05-11 dev2 -> ht65b-dev2
2026-05-09 dev1 -> ht659-dev1
2026-12-31 dev1 -> ht612v-dev1
```

## 创建分支

从 `main` 使用脚本创建：

```powershell
.\scripts\new-branch.ps1
```

创建同一天第二个分支：

```powershell
.\scripts\new-branch.ps1 -Dev 2
```

创建指定分支名：

```powershell
.\scripts\new-branch.ps1 ht65b-dev3
```

默认情况下，脚本会拒绝从脏工作区创建分支。确实要保留当前改动并切分支时，使用 `-AllowDirty`。

## 合并规则

常规改动不要直接提交到 `main`。

流程：

```text
main -> review 分支 -> commit -> review -> merge 到 main
```

能 fast-forward 时使用：

```powershell
git switch main
git merge --ff-only <branch-name>
```

## Pre-Commit 检查

每个 clone 安装一次本地 Git hook：

```powershell
.\scripts\install-git-hooks.ps1
```

手动运行同一套检查：

```powershell
.\scripts\pre-commit.ps1
```

pre-commit 会检查分支命名、阻止常见密钥文件、扫描 staged diff 里的疑似密钥、检查 Go 文件名、验证 `gofmt`，并运行 `go vet ./...` 和 `go test ./...`。

如果是少数仓库维护提交，确实需要在 `main` 上手动运行检查，可以使用：

```powershell
.\scripts\pre-commit.ps1 -AllowMain
```
