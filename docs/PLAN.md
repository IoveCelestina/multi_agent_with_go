# Multi-Agent Go 项目计划

目标：构建一个 Go 版本的 multi-agent 项目。这个项目优先服务学习：先跑通最小闭环，再逐步深入 provider、tool calling、coordinator、持久化、resume、并发和 reviewer。

学习重点不是堆功能，而是理解 agent 工程里的真实难点：

- 可观测性：agent 失败后如何 debug。
- 状态机和幂等性：中断、恢复、重复 tool call 怎么处理。
- agent 间协议：reviewer、重派、revision task 怎么表达。
- 运行时边界：streaming、取消、tool sandbox、错误传播怎么设计。

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

## 学习优先级

不要为了更快做出 UI 跳过阶段 6/7/10：

- 阶段 6 Event log + SQLite 是可观测性训练，决定后续能不能 debug agent。
- 阶段 7 Resume checkpoint 是状态机和幂等性训练，是 agent 工程里非常核心的可靠性问题。
- 阶段 10 Reviewer + 状态机 + 重派是 multi-agent 协议设计训练，比单纯多调几次模型更重要。

Bubble Tea REPL 放在阶段 8，不能提前替代 6/7。

## 依赖策略

不再追求长期零第三方依赖。判断标准：

```text
这块逻辑是 agent 工程独有的吗？
```

- 是：自己写，例如 Provider 接口、tool loop、event log、AgentRunner、Coordinator、状态机、resume 协议、tool registry、sandbox。
- 不是：优先用成熟库，例如 YAML、CLI 框架、SQLite driver、通用 SSE parser。

近期依赖取舍：

| 用库 | 自己写 |
| --- | --- |
| `gopkg.in/yaml.v3` 解析配置 | Provider 接口和 provider 归一化事件 |
| Cobra 做 CLI 子命令 | AgentRunner、tool loop、Coordinator |
| SQLite driver | event schema、resume 协议 |
| 通用 SSE 解析辅助 | tool registry、read_file sandbox |

## 阶段笔记

每个阶段完成后写一篇短笔记，放在：

```text
docs/journal/phase-N.md
```

每篇控制在 200-400 字，回答：

- 这个阶段最难的决定是什么？为什么这样选？
- 哪里和开工前预期不一样？
- 和 OpenAI Swarm / LangGraph / Claude Code SDK 的类似设计有什么差异？

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

状态：已完成真实调用验收。

## 下一步

进入阶段 2 前先做两个基础动作：

1. 用 `gopkg.in/yaml.v3` 替换当前手写配置解析器。
2. 补 `docs/journal/phase-1.md`，记录阶段 1 的 streaming、取消、tool call delta、`.env` 和 pre-commit 经验。

然后进入阶段 2：接入 Kimi，验证同一套 `agentctl chat` 是否可以切换 provider。

## 近期实现原则

- Provider 接口先保持小，只抽象 streaming chat 的最小公共能力。
- AgentRunner 必须支持 tool loop：模型输出 tool call、执行 tool、把 tool result 回传模型、再生成最终回答。
- Coordinator 初版用同步流程，不急着并发。
- 并发前先固定 event log，否则后续难以 debug。
- Reviewer 初版只做结构化判定，不做复杂 debate。
- 每个阶段结束后补 journal，再进入下一阶段。
