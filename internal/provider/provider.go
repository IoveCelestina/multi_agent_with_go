package provider

import (
	"context"
	"encoding/json"
)

// 表示模型对话消息角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// 表示发送给模型 provider 的一条消息。
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

// 表示模型请求的一次完整工具调用。
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// 描述 provider 可以暴露给模型的一个工具。
type ToolSpec struct {
	Name        string
	Description string
	JSONSchema  json.RawMessage
}

// 归一化后的流式聊天请求。
type ChatRequest struct {
	Messages    []Message
	Tools       []ToolSpec
	Temperature *float64
	MaxTokens   int
}

// 表示 provider 流式事件类型。
type EventKind int

const (
	EventText EventKind = iota
	EventToolCall
	EventDone
	EventError
)

// 表示模型响应停止的原因。
type FinishReason string

const (
	FinishStop      FinishReason = "stop"
	FinishToolCalls FinishReason = "tool_calls"
	FinishLength    FinishReason = "length"
)

// 归一化后的 provider 流式事件。
type Event struct {
	Kind         EventKind
	TextDelta    string
	ToolCall     *ToolCall
	FinishReason FinishReason
	Err          error
}

// 模型 provider 的最小公共接口。
type Provider interface {
	Name() string
	Stream(ctx context.Context, req ChatRequest) (<-chan Event, error)
}
