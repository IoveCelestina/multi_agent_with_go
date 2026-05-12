# AgentRunner 设计

AgentRunner 是单个 agent 的运行循环。它负责维护消息历史、调用一个 Provider、执行允许的工具，并在模型给出最终回答或触发安全上限时停止。

这份文档定义第一版实现契约。它暂时不处理 Coordinator、持久化、resume 和并行工具执行；这些属于后续阶段。

## 目标

- 跑通单个 agent 的 streaming chat + tool calling 循环。
- 让 AgentRunner 看不到 Provider 的原始协议细节。
- 让 Tool 自己负责参数解析和业务校验，AgentRunner 不做 JSON Schema 运行时校验。
- 保留足够结构化的运行事件，为后续 event log 和 resume 做准备。
- 明确定义取消、错误和 max rounds 的行为。

## 非目标

- 不做多 agent 协调。
- 不做 SQLite event log。
- 不做 resume checkpoint。
- 不做并行工具执行。
- 不做 JSON Schema 运行时校验。

## 包结构

第一版包名：

```text
internal/agent
```

建议公开类型：

```go
type Runner struct {
	provider provider.Provider
	tools    *tool.Registry
	config   Config
}

type Config struct {
	Name         string
	SystemPrompt string
	ToolNames    []string
	MaxRounds    int
	Temperature  *float64
	MaxTokens    int
}

type Input struct {
	UserMessage string
	Messages    []provider.Message
}

type Output struct {
	FinalText string
	Messages  []provider.Message
	Rounds    int
}

type Event struct {
	Kind     EventKind
	Text     string
	ToolCall *provider.ToolCall
	ToolName string
	IsError  bool
	Message  string
}
```

`Input.UserMessage` 用于 CLI 和 demo 的常见场景。`Input.Messages` 用于测试和后续 Coordinator 传递历史。两者同时存在时，先使用 `Messages`，再把 `UserMessage` 追加为 user message。

## 事件输出

Runner 需要暴露流式进度，但不能依赖具体 UI 包。

第一版使用回调：

```go
type EventSink func(Event)

func (r *Runner) Run(ctx context.Context, input Input, sink EventSink) (Output, error)
```

如果 `sink` 为 nil，Runner 仍然正常运行。Event 是尽力通知，不能反过来控制核心状态转换。

第一版事件类型：

```go
const (
	EventTextDelta EventKind = iota
	EventToolCall
	EventToolResult
	EventRoundDone
)
```

后续 event log 阶段可以把这些事件映射成 SQLite 事件记录。

## 消息历史

Runner 按下面顺序构建消息历史：

1. agent config 里的 system prompt。
2. `Input.Messages` 里已有的历史。
3. `Input.UserMessage` 生成的 user message。
4. Provider 返回后的 assistant message。
5. tool result message。

只有当输入历史里没有 system message 时，Runner 才自动追加 system prompt。这样 Coordinator 以后传入显式历史时，不会重复注入 system prompt。

每次 Provider stream 结束后，Runner 都要追加 assistant message。即使 assistant 文本为空也要追加，因为 tool calling 协议要求 tool result 紧跟声明 tool call 的 assistant message：

```go
provider.Message{
	Role:      provider.RoleAssistant,
	Content:   assistantText,
	ToolCalls: toolCalls,
}
```

每个工具结果追加为：

```go
provider.Message{
	Role:       provider.RoleTool,
	ToolCallID: call.ID,
	Name:       call.Name,
	Content:    result.Content,
}
```

## 工具选择

Agent YAML 只列出允许使用的工具名。Runner 在第一次调用 Provider 前，从 Registry 获取工具声明：

```go
specs, err := registry.Specs(config.ToolNames)
```

未知工具名是配置错误，run 开始前直接失败。

Provider 只接收该 agent 被允许使用的工具声明。如果模型仍然返回未知工具调用，Runner 不崩溃，而是追加一个错误 tool result，让模型有机会自行修正：

```text
tool "x" is not available to this agent
```

## 循环算法

伪代码：

```go
for round := 1; round <= maxRounds; round++ {
	ch, err := prov.Stream(ctx, provider.ChatRequest{
		Messages: msgs,
		Tools: specs,
		Temperature: cfg.Temperature,
		MaxTokens: cfg.MaxTokens,
	})
	if err != nil { return output, err }

	var assistantText string
	var toolCalls []provider.ToolCall
	var finish provider.FinishReason

	for ev := range ch {
		switch ev.Kind {
		case provider.EventText:
			assistantText += ev.TextDelta
			sink(Event{Kind: EventTextDelta, Text: ev.TextDelta})
		case provider.EventToolCall:
			toolCalls = append(toolCalls, *ev.ToolCall)
			sink(Event{Kind: EventToolCall, ToolCall: ev.ToolCall})
		case provider.EventDone:
			finish = ev.FinishReason
		case provider.EventError:
			return output, ev.Err
		}
	}

	msgs = append(msgs, assistantMessage(assistantText, toolCalls))

	if finish != provider.FinishToolCalls || len(toolCalls) == 0 {
		return Output{FinalText: assistantText, Messages: msgs, Rounds: round}, nil
	}

	for _, call := range toolCalls {
		msgs = append(msgs, executeTool(ctx, call))
	}
}

return output, ErrMaxRounds
```

如果 Provider channel 关闭但没有发送 `EventDone`，Runner 只在“已经收到文本且没有 tool call”时把它当作普通结束；否则返回错误。Provider 理论上应该发送 `EventDone`，但 OpenAI-compatible 服务并不总是严格一致，所以 Runner 需要保留这个保护。

## 工具执行

第一版工具调用串行执行，按 Provider 返回顺序处理。并行执行等到并发阶段再做。

规则：

- AgentRunner 不做 JSON Schema 参数校验。
- Tool 接收 raw JSON 参数。
- `context.Canceled` 和 `context.DeadlineExceeded` 直接停止 run。
- 普通 tool error 转成 `IsError` tool result。
- Tool 返回 `Result{IsError: true}` 时，也作为普通 tool message 回传给模型。

工具结果内容应尽量简洁。第一版不做全局截断；每个 tool 自己负责输出上限，例如 `read_file.max_bytes`。

## Max Rounds

`MaxRounds` 在创建 Runner 前解析完成：

1. agent override
2. runtime default
3. 代码默认值 `10`

有效范围是 `1 <= max_rounds <= 32`。

一轮表示一次 Provider response，后面可以跟一批串行 tool call。如果最后一轮后模型仍继续请求工具，Runner 返回 `ErrMaxRounds`，并带上目前为止累计的完整消息历史。

## 取消

Runner 把 `ctx` 传给 Provider 和 Tool。

取消规则：

- Provider 调用前 `ctx` 已取消，则返回 `ctx.Err()`。
- Provider channel 因调用方取消而关闭，则返回 `ctx.Err()`。
- Tool 执行返回 `context.Canceled` 或 `DeadlineExceeded`，则直接返回该错误。
- 不把调用方主动取消转换成 tool result。

## 错误

第一版哨兵错误：

```go
var ErrMaxRounds = errors.New("agent runner reached max rounds")
var ErrMissingProviderDone = errors.New("provider stream ended without done event")
```

错误需要带足上下文：

- provider 名
- agent 名
- round 编号
- 相关 tool 名和 tool call ID

## 测试计划

使用 fake Provider 和 fake Tool，不调用真实模型 API。

必须覆盖：

- 没有 tool call 时直接返回 final text。
- text delta 按顺序发送到 sink。
- 一个 tool call 执行后，tool result 在下一轮回传给 Provider。
- 未知 tool call 变成错误 tool message。
- tool 参数错误变成错误 tool message。
- `context.Canceled` 会停止 run。
- max rounds 返回 `ErrMaxRounds`。
- Provider stream error 会返回错误。
- Provider channel close without done 按文档规则处理。

## 后续扩展

Event log 阶段：

- 持久化 round start/done。
- 持久化 provider text delta 或压缩后的文本。
- 持久化 tool call 和 tool result。
- 持久化带 round/tool 元数据的错误。

Resume 阶段：

- 分配稳定的 run/task/agent/tool ID。
- 避免重复执行已完成 tool call。
- 从持久化事件重建 provider message history。

Coordinator 阶段：

- 把 `Output.Messages` 或摘要后的输出传给下一个 agent。
- 保持 AgentRunner 单 agent、可复用。
