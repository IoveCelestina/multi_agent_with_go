# Multi Agent Go Project Plan

目标：构建一个 Go 版本的 multi-agent 项目，先跑通最小闭环，再逐步增加 provider、tool calling、coordinator、持久化、并发和 reviewer。

## 开发节奏

| 阶段 | 工期 | 验收物 |
| --- | ---: | --- |
| 0. 骨架 + go.mod + 一份 YAML 配置 | 0.5d | `agentctl version` 能跑 |
| 1. Provider + DeepSeek streaming | 1d | `agentctl chat --provider deepseek` 单轮流式输出，支持 `Ctrl+C` 取消 |
| 2. Kimi 接入，验证 Provider 抽象 | 1d | 同一套 chat 代码可切换 DeepSeek/Kimi |
| 3. AgentRunner + tool loop + read_file | 2d | Agent 能调用 `read_file`，tool result 能回传模型继续生成 |
| 4. Researcher -> Writer 同步 Coordinator | 2d | 研究 + 写作 demo 跑通，并能打印每个 agent 的输入/输出摘要 |
| 5. Cobra CLI: chat/run/version | 1d | `agentctl run examples/research_writer.yaml` 能跑 |
| 6. Event log + SQLite 持久化 | 1.5d | run/task/agent/tool 事件可落库和查询 |
| 7. Resume checkpoint | 1d | 杀进程后能从 last completed step 继续，不重复执行已完成 tool call |
| 8. Bubble Tea REPL | 1.5d | 交互模式支持流式输出 |
| 9. 并发 Researcher + errgroup | 1.5d | 多 Researcher 可并行执行，支持 timeout 和错误汇总 |
| 10. Reviewer + 状态机 + 重派 | 2d | Reviewer 不通过时 Coordinator 能创建 revision task 并重派 |
| 11. README + examples + asciinema | 1d | 完整 README、示例配置和终端演示 |

## 当前阶段

### 阶段 1：Provider + DeepSeek streaming

范围：

- 读取 `configs/agents.yaml` 里的 provider 配置
- 实现 DeepSeek OpenAI-compatible `/chat/completions` streaming provider
- 新增 `agentctl chat --provider deepseek` 单轮流式输出
- 支持 `Ctrl+C` 取消请求
- 保持 provider 接口可复用，后续 Kimi 接入不改 CLI 主流程

验收：

```powershell
go run ./cmd/agentctl chat --provider deepseek "hello"
```

需要先设置：

```powershell
$env:DEEPSEEK_API_KEY="<your api key>"
```

## 近期实现原则

- Provider 接口先保持小，只抽象 streaming chat 的最小公共能力。
- AgentRunner 必须支持 tool loop：模型输出 tool call、执行 tool、把 tool result 回传模型、再生成最终回答。
- Coordinator 初版用同步流程，不急着并发。
- 并发前先固定 event log，否则后续难以 debug。
- Reviewer 初版只做结构化判定，不做复杂 debate。
