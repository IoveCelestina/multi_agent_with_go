# Provider And Tool Design

This document defines the initial Provider and Tool boundary for the multi-agent
runtime.

## Goals

- Provider abstracts streaming chat and tool calling across model vendors.
- Provider hides raw SSE/protocol frames from AgentRunner.
- AgentRunner consumes normalized events only: text delta, completed tool call,
  done, or error.
- Tools are implemented and registered in Go. YAML only controls which tools an
  agent may use.
- Runtime keeps the first implementation small and dependency-light.

## Provider Interface

Provider exposes the smallest common surface:

```go
type Provider interface {
	Name() string
	Stream(ctx context.Context, req ChatRequest) (<-chan Event, error)
}
```

`Stream` error semantics:

- Return `error` when streaming cannot start, such as invalid request setup or
  HTTP request creation failure.
- Send `EventError` after the stream has started, such as SSE decode failure,
  provider-side stream errors, or interrupted responses.
- When `ctx` is canceled by the caller, Provider may close the channel without
  sending `EventError`; caller-initiated cancellation is already visible through
  the caller's context.
- Provider owns and closes the event channel.
- Provider must stop its goroutine when `ctx` is canceled.
- Provider should send exactly one `EventDone` for a completed response. If an
  upstream stream ends with `[DONE]` before an explicit finish reason, Provider
  should send a best-effort `EventDone` with `FinishStop`.

Tool calls are assembled inside provider implementations. AgentRunner receives a
complete `ToolCall` with raw JSON arguments, not streaming argument deltas.

`ToolCall.Arguments` stays as raw JSON. Provider validates that the argument
string is complete enough to forward, but does not unmarshal into tool-specific
types.

## Tool Interface

Tools are registered in Go:

```go
type Tool interface {
	Name() string
	Description() string
	JSONSchema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (Result, error)
}
```

`JSONSchema` is a model-facing hint. AgentRunner does not run JSON Schema
validation. Each tool unmarshals into its own argument struct and performs
business validation.

Tool execution error semantics:

- Invalid arguments and domain failures should normally return
  `Result{IsError: true, Content: "..."}`
- `context.Canceled` and `context.DeadlineExceeded` should return `error` so the
  run can stop promptly.
- AgentRunner should convert ordinary tool errors into error tool messages unless
  the run context has been canceled.

## Registry

The tool registry is the only source of available tools. Agent YAML lists names
from the registry:

```yaml
agents:
  researcher:
    tools:
      - read_file
```

Unknown tool names are configuration errors. Registry must not silently skip
them.

Registry validates tool definitions at registration time:

- name is non-empty and uses `^[a-z][a-z0-9_]*$`
- name is unique
- description is non-empty
- schema is valid JSON

## Read File Sandbox

`read_file` uses explicit filesystem roots:

```yaml
tools:
  read_file:
    roots:
      - ./
    max_bytes: 65536
```

Rules:

- Roots are normalized to absolute paths during initialization.
- Roots must pass `filepath.EvalSymlinks`; failure is a startup/config error.
- User paths may be absolute or relative to the process working directory.
- User paths must pass `filepath.EvalSymlinks`; failure is returned as an
  `IsError` tool result.
- Never fall back to a cleaned but unresolved path after `EvalSymlinks` failure.
- Resolved paths must be equal to a root or inside a root with a real path
  boundary. `C:\repo` must not allow `C:\repo2`.
- Directory reads are rejected.
- Reads are capped by `max_bytes`.

## Agent Runner Loop

Initial tool calls run serially. Parallel tool execution is deferred until the
concurrency phase.

AgentRunner appends the assistant message containing the model text and all tool
calls before appending tool result messages. This preserves the provider message
contract:

```go
provider.Message{
	Role:      provider.RoleAssistant,
	Content:   assistantText,
	ToolCalls: toolCalls,
}
```

Each tool result becomes a `RoleTool` message with the matching `ToolCallID`.

## Max Rounds

`max_rounds` belongs to AgentRunner runtime control, not Provider.

Configuration policy:

- `runtime.max_rounds` sets the global default.
- `agents.<name>.max_rounds` can override the global default.
- If neither is configured, code uses a default of `10`.
- Valid range is `1 <= max_rounds <= 32`.

Example:

```yaml
runtime:
  max_rounds: 10

agents:
  researcher:
    max_rounds: 8
```
