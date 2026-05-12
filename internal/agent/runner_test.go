package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ht/multi_agent/internal/provider"
	"github.com/ht/multi_agent/internal/tool"
)

func TestRunReturnsFinalTextWithoutToolCalls(t *testing.T) {
	prov := &fakeProvider{
		rounds: [][]provider.Event{{
			{Kind: provider.EventText, TextDelta: "你好"},
			{Kind: provider.EventText, TextDelta: "，世界"},
			{Kind: provider.EventDone, FinishReason: provider.FinishStop},
		}},
	}
	runner := newTestRunner(t, prov, nil, Config{
		Name:         "writer",
		SystemPrompt: "你是写作者",
	})

	var events []Event
	out, err := runner.Run(context.Background(), Input{UserMessage: "hi"}, func(event Event) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if out.FinalText != "你好，世界" {
		t.Fatalf("FinalText = %q, want 你好，世界", out.FinalText)
	}
	if out.Rounds != 1 {
		t.Fatalf("Rounds = %d, want 1", out.Rounds)
	}
	if len(out.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(out.Messages))
	}
	if out.Messages[0].Role != provider.RoleSystem {
		t.Fatalf("first role = %q, want system", out.Messages[0].Role)
	}
	if collectTextEvents(events) != "你好，世界" {
		t.Fatalf("text events = %q", collectTextEvents(events))
	}
}

func TestRunExecutesToolAndSendsResultToNextRound(t *testing.T) {
	call := provider.ToolCall{ID: "call_1", Name: "echo", Arguments: `{"text":"ok"}`}
	prov := &fakeProvider{
		rounds: [][]provider.Event{
			{
				{Kind: provider.EventToolCall, ToolCall: &call},
				{Kind: provider.EventDone, FinishReason: provider.FinishToolCalls},
			},
			{
				{Kind: provider.EventText, TextDelta: "完成"},
				{Kind: provider.EventDone, FinishReason: provider.FinishStop},
			},
		},
	}
	registry := tool.NewRegistry()
	if err := registry.Register(fakeTool{name: "echo", content: "tool ok"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	runner := newTestRunner(t, prov, registry, Config{Name: "researcher", ToolNames: []string{"echo"}})

	out, err := runner.Run(context.Background(), Input{UserMessage: "run"}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if out.FinalText != "完成" {
		t.Fatalf("FinalText = %q, want 完成", out.FinalText)
	}
	if len(prov.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(prov.requests))
	}

	secondMessages := prov.requests[1].Messages
	if len(secondMessages) < 3 {
		t.Fatalf("second request messages = %d, want at least 3", len(secondMessages))
	}
	toolMsg := secondMessages[len(secondMessages)-1]
	if toolMsg.Role != provider.RoleTool {
		t.Fatalf("last second request role = %q, want tool", toolMsg.Role)
	}
	if toolMsg.ToolCallID != "call_1" || toolMsg.Name != "echo" || toolMsg.Content != "tool ok" {
		t.Fatalf("tool message = %#v", toolMsg)
	}
}

func TestRunConvertsUnknownToolCallToErrorToolMessage(t *testing.T) {
	call := provider.ToolCall{ID: "call_1", Name: "missing", Arguments: `{}`}
	prov := &fakeProvider{
		rounds: [][]provider.Event{
			{
				{Kind: provider.EventToolCall, ToolCall: &call},
				{Kind: provider.EventDone, FinishReason: provider.FinishToolCalls},
			},
			{
				{Kind: provider.EventText, TextDelta: "已修正"},
				{Kind: provider.EventDone, FinishReason: provider.FinishStop},
			},
		},
	}
	runner := newTestRunner(t, prov, nil, Config{Name: "agent"})

	var events []Event
	_, err := runner.Run(context.Background(), Input{UserMessage: "run"}, func(event Event) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	toolMsg := prov.requests[1].Messages[len(prov.requests[1].Messages)-1]
	if toolMsg.Role != provider.RoleTool {
		t.Fatalf("tool message role = %q, want tool", toolMsg.Role)
	}
	if !strings.Contains(toolMsg.Content, "not available") {
		t.Fatalf("tool message content = %q, want not available", toolMsg.Content)
	}
	if !lastToolResultIsError(events) {
		t.Fatal("tool result event did not report error")
	}
}

func TestRunConvertsToolErrorToErrorToolMessage(t *testing.T) {
	call := provider.ToolCall{ID: "call_1", Name: "echo", Arguments: `{}`}
	prov := &fakeProvider{
		rounds: [][]provider.Event{
			{
				{Kind: provider.EventToolCall, ToolCall: &call},
				{Kind: provider.EventDone, FinishReason: provider.FinishToolCalls},
			},
			{
				{Kind: provider.EventText, TextDelta: "继续"},
				{Kind: provider.EventDone, FinishReason: provider.FinishStop},
			},
		},
	}
	registry := tool.NewRegistry()
	if err := registry.Register(fakeTool{name: "echo", err: errors.New("boom")}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	runner := newTestRunner(t, prov, registry, Config{Name: "agent", ToolNames: []string{"echo"}})

	_, err := runner.Run(context.Background(), Input{UserMessage: "run"}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	toolMsg := prov.requests[1].Messages[len(prov.requests[1].Messages)-1]
	if !strings.Contains(toolMsg.Content, "boom") {
		t.Fatalf("tool message content = %q, want boom", toolMsg.Content)
	}
}

func TestRunStopsOnContextCanceled(t *testing.T) {
	registry := tool.NewRegistry()
	if err := registry.Register(fakeTool{name: "echo", err: context.Canceled}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	call := provider.ToolCall{ID: "call_1", Name: "echo", Arguments: `{}`}
	prov := &fakeProvider{
		rounds: [][]provider.Event{{
			{Kind: provider.EventToolCall, ToolCall: &call},
			{Kind: provider.EventDone, FinishReason: provider.FinishToolCalls},
		}},
	}
	runner := newTestRunner(t, prov, registry, Config{Name: "agent", ToolNames: []string{"echo"}})

	_, err := runner.Run(context.Background(), Input{UserMessage: "run"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestRunReturnsMaxRounds(t *testing.T) {
	call := provider.ToolCall{ID: "call_1", Name: "echo", Arguments: `{}`}
	prov := &fakeProvider{
		rounds: [][]provider.Event{{
			{Kind: provider.EventToolCall, ToolCall: &call},
			{Kind: provider.EventDone, FinishReason: provider.FinishToolCalls},
		}},
	}
	registry := tool.NewRegistry()
	if err := registry.Register(fakeTool{name: "echo", content: "ok"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	runner := newTestRunner(t, prov, registry, Config{Name: "agent", ToolNames: []string{"echo"}, MaxRounds: 1})

	out, err := runner.Run(context.Background(), Input{UserMessage: "run"}, nil)
	if !errors.Is(err, ErrMaxRounds) {
		t.Fatalf("Run() error = %v, want ErrMaxRounds", err)
	}
	if out.Rounds != 1 {
		t.Fatalf("Rounds = %d, want 1", out.Rounds)
	}
}

func TestRunReturnsProviderStreamError(t *testing.T) {
	prov := &fakeProvider{
		rounds: [][]provider.Event{{
			{Kind: provider.EventError, Err: errors.New("stream failed")},
		}},
	}
	runner := newTestRunner(t, prov, nil, Config{Name: "agent"})

	_, err := runner.Run(context.Background(), Input{UserMessage: "run"}, nil)
	if err == nil || !strings.Contains(err.Error(), "stream failed") {
		t.Fatalf("Run() error = %v, want stream failed", err)
	}
}

func TestRunHandlesChannelCloseWithoutDoneAfterText(t *testing.T) {
	prov := &fakeProvider{
		rounds: [][]provider.Event{{
			{Kind: provider.EventText, TextDelta: "直接结束"},
		}},
	}
	runner := newTestRunner(t, prov, nil, Config{Name: "agent"})

	out, err := runner.Run(context.Background(), Input{UserMessage: "run"}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out.FinalText != "直接结束" {
		t.Fatalf("FinalText = %q, want 直接结束", out.FinalText)
	}
}

func TestRunReturnsErrorWhenChannelClosesWithoutDoneAndText(t *testing.T) {
	prov := &fakeProvider{rounds: [][]provider.Event{{}}}
	runner := newTestRunner(t, prov, nil, Config{Name: "agent"})

	_, err := runner.Run(context.Background(), Input{UserMessage: "run"}, nil)
	if !errors.Is(err, ErrMissingProviderDone) {
		t.Fatalf("Run() error = %v, want ErrMissingProviderDone", err)
	}
}

func TestRunDoesNotDuplicateSystemPrompt(t *testing.T) {
	prov := &fakeProvider{
		rounds: [][]provider.Event{{
			{Kind: provider.EventText, TextDelta: "ok"},
			{Kind: provider.EventDone, FinishReason: provider.FinishStop},
		}},
	}
	runner := newTestRunner(t, prov, nil, Config{Name: "agent", SystemPrompt: "新系统提示"})

	_, err := runner.Run(context.Background(), Input{
		Messages: []provider.Message{{Role: provider.RoleSystem, Content: "已有系统提示"}},
	}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	messages := prov.requests[0].Messages
	if len(messages) == 0 || messages[0].Content != "已有系统提示" {
		t.Fatalf("first message = %#v, want existing system prompt", messages)
	}
}

func newTestRunner(t *testing.T, prov provider.Provider, registry *tool.Registry, cfg Config) *Runner {
	t.Helper()

	runner, err := NewRunner(prov, registry, cfg)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	return runner
}

type fakeProvider struct {
	rounds   [][]provider.Event
	requests []provider.ChatRequest
	err      error
}

func (f *fakeProvider) Name() string {
	return "fake"
}

func (f *fakeProvider) Stream(ctx context.Context, req provider.ChatRequest) (<-chan provider.Event, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.requests = append(f.requests, req)

	index := len(f.requests) - 1
	events := []provider.Event{
		{Kind: provider.EventText, TextDelta: "default"},
		{Kind: provider.EventDone, FinishReason: provider.FinishStop},
	}
	if index < len(f.rounds) {
		events = f.rounds[index]
	}

	ch := make(chan provider.Event, len(events))
	for _, event := range events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

type fakeTool struct {
	name    string
	content string
	isError bool
	err     error
}

func (f fakeTool) Name() string {
	return f.name
}

func (f fakeTool) Description() string {
	return "测试工具"
}

func (f fakeTool) JSONSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (f fakeTool) Execute(ctx context.Context, args json.RawMessage) (tool.Result, error) {
	if f.err != nil {
		return tool.Result{}, f.err
	}
	return tool.Result{Content: f.content, IsError: f.isError}, nil
}

func collectTextEvents(events []Event) string {
	var text string
	for _, event := range events {
		if event.Kind == EventTextDelta {
			text += event.Text
		}
	}
	return text
}

func lastToolResultIsError(events []Event) bool {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind == EventToolResult {
			return events[i].IsError
		}
	}
	return false
}
