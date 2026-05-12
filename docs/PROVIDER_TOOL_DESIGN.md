# Provider 和 Tool 设计

本文定义 multi-agent 运行时里 Provider 和 Tool 的第一版边界。

## 目标

- Provider 抽象 streaming chat 和 tool calling，屏蔽不同模型厂商协议差异。
- Provider 不把原始 SSE 或协议帧暴露给 AgentRunner。
- AgentRunner 只消费归一化事件：文本增量、完整工具调用、结束或错误。
- Tool 使用 Go 代码实现和注册。YAML 只控制某个 agent 允许使用哪些工具。
- 第一版运行时保持小而清晰，避免在核心边界还没稳定前过度抽象。

## Provider 接口

Provider 暴露最小公共接口：

```go
type Provider interface {
	Name() string
	Stream(ctx context.Context, req ChatRequest) (<-chan Event, error)
}
```

`Stream` 的错误语义：

- 如果流还没启动就失败，例如请求构造失败或 HTTP request 创建失败，直接返回 `error`。
- 如果流已经启动后失败，例如 SSE 解码失败、服务端流式错误或响应中断，通过 `EventError` 发送。
- 如果调用方主动取消 `ctx`，Provider 可以直接关闭 channel，不必发送 `EventError`；调用方本来就能从自己的 context 看到取消。
- Provider 拥有并负责关闭事件 channel。
- `ctx` 取消后，Provider 必须停止内部 goroutine。
- Provider 对一次完成响应应只发送一个 `EventDone`。如果上游在显式 finish reason 前直接发送 `[DONE]`，Provider 应尽力发送 `FinishStop` 作为兜底结束事件。

工具调用在 Provider 内部组装。AgentRunner 只接收完整 `ToolCall`，不接收工具参数的流式增量。

`ToolCall.Arguments` 保留 raw JSON。Provider 只负责把参数字符串组装完整，不把它 unmarshal 成工具专属类型。

## Tool 接口

Tool 使用 Go 注册：

```go
type Tool interface {
	Name() string
	Description() string
	JSONSchema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (Result, error)
}
```

`JSONSchema` 只是给模型看的提示。AgentRunner 不做 JSON Schema 运行时校验。每个 Tool 自己把参数 unmarshal 到具体结构体，并执行业务校验。

Tool 执行错误语义：

- 参数非法和领域失败通常返回 `Result{IsError: true, Content: "..."}`。
- `context.Canceled` 和 `context.DeadlineExceeded` 应返回 `error`，让 run 尽快停止。
- 除 run context 已取消外，AgentRunner 应把普通 tool error 转成错误 tool message 回传给模型。

## Registry

Tool Registry 是可用工具的唯一来源。Agent YAML 只列出 Registry 里的工具名：

```yaml
agents:
  researcher:
    tools:
      - read_file
```

未知工具名是配置错误。Registry 不允许静默跳过未知工具。

Registry 在注册工具时校验：

- name 非空，并符合 `^[a-z][a-z0-9_]*$`
- name 唯一
- description 非空
- schema 是合法 JSON

## read_file 沙箱

`read_file` 使用显式文件系统 roots：

```yaml
tools:
  read_file:
    roots:
      - ./
    max_bytes: 65536
```

规则：

- 初始化时把 Roots 归一化为绝对路径。
- Roots 必须通过 `filepath.EvalSymlinks`；失败就是启动或配置错误。
- 用户 path 可以是绝对路径，也可以是相对当前进程工作目录的相对路径。
- 用户 path 必须通过 `filepath.EvalSymlinks`；失败时返回 `IsError` tool result。
- `EvalSymlinks` 失败后绝不能 fallback 到只 clean 过的路径。
- 解析后的路径必须等于某个 root，或位于 root 内部，并且必须有真实路径边界。`C:\repo` 不能放行 `C:\repo2`。
- 拒绝读取目录。
- 读取大小受 `max_bytes` 限制。

## AgentRunner 循环

第一版工具调用串行执行。并行工具执行等到并发阶段再做。

AgentRunner 在追加 tool result message 前，必须先追加包含模型文本和所有 tool call 的 assistant message。这样才能保留 provider 消息协议：

```go
provider.Message{
	Role:      provider.RoleAssistant,
	Content:   assistantText,
	ToolCalls: toolCalls,
}
```

每个 tool result 都变成一条带匹配 `ToolCallID` 的 `RoleTool` message。

## Max Rounds

`max_rounds` 属于 AgentRunner 的运行控制参数，不属于 Provider。

配置策略：

- `runtime.max_rounds` 设置全局默认值。
- `agents.<name>.max_rounds` 可以覆盖全局默认值。
- 如果两者都没配置，代码默认使用 `10`。
- 有效范围是 `1 <= max_rounds <= 32`。

示例：

```yaml
runtime:
  max_rounds: 10

agents:
  researcher:
    max_rounds: 8
```
