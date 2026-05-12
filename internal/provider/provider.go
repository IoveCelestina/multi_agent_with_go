package provider

import (
	"context"
	"encoding/json"
)

// Role identifies a model conversation message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one message sent to a model provider.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

// ToolCall is one completed tool call requested by a model.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ToolSpec describes one tool that a provider may expose to the model.
type ToolSpec struct {
	Name        string
	Description string
	JSONSchema  json.RawMessage
}

// ChatRequest is a normalized streaming chat request.
type ChatRequest struct {
	Messages    []Message
	Tools       []ToolSpec
	Temperature *float64
	MaxTokens   int
}

// EventKind identifies the kind of streamed provider event.
type EventKind int

const (
	EventText EventKind = iota
	EventToolCall
	EventDone
	EventError
)

// FinishReason identifies why a model response stopped.
type FinishReason string

const (
	FinishStop      FinishReason = "stop"
	FinishToolCalls FinishReason = "tool_calls"
	FinishLength    FinishReason = "length"
)

// Event is one normalized streaming provider event.
type Event struct {
	Kind         EventKind
	TextDelta    string
	ToolCall     *ToolCall
	FinishReason FinishReason
	Err          error
}

// Provider is the smallest shared model provider interface.
type Provider interface {
	Name() string
	Stream(ctx context.Context, req ChatRequest) (<-chan Event, error)
}
