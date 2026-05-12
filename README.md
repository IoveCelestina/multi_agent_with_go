# multi_agent_with_go

Go 版本的 multi-agent 实验项目。目标是先跑通最小闭环，再逐步增加 provider、tool calling、coordinator、持久化、并发和 reviewer。

## 当前状态

已完成：

- Go CLI 骨架：`agentctl version`
- DeepSeek streaming chat：`agentctl chat --provider deepseek`
- `.env` 本地密钥加载
- Provider 基础抽象
- Tool 基础抽象和 `read_file` 沙箱工具
- pre-commit 本地检查

下一阶段：

- 接入 Kimi，验证同一套 chat 代码可以切换 provider。

## 快速开始

检查版本：

```powershell
go run ./cmd/agentctl version
```

配置本地密钥：

```powershell
Copy-Item .env.example .env
```

编辑 `.env`，填入真实 key：

```env
DEEPSEEK_API_KEY=your_key_here
KIMI_API_KEY=
OPENAI_API_KEY=
```

运行 DeepSeek chat：

```powershell
go run ./cmd/agentctl chat --provider deepseek "用一句话介绍这个项目"
```

流式输出过程中可以按 `Ctrl+C` 取消请求。

## 开发检查

安装本地 Git hook：

```powershell
.\scripts\install-git-hooks.ps1
```

手动运行检查：

```powershell
.\scripts\pre-commit.ps1
```

检查内容包括：

- 分支命名
- 阻止提交 `.env` 和私钥类文件
- staged diff 密钥扫描
- Go 文件名规范
- `gofmt`
- `go vet ./...`
- `go test ./...`

## 配置

主配置文件：

```text
configs/agents.yaml
```

当前阶段主要使用其中的 `providers` 配置。`agents`、`tools`、`workflows` 已保留为后续阶段使用。

## 文档

- [开发计划](docs/PLAN.md)
- [代码风格](docs/CODING_STYLE.md)
- [Git 流程](docs/GIT_WORKFLOW.md)
- [Provider 和 Tool 设计](docs/PROVIDER_TOOL_DESIGN.md)
