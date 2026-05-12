package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/ht/multi_agent/internal/provider"
	"github.com/ht/multi_agent/internal/tool"
)

const defaultMaxRounds = 10

// ErrMaxRounds 表示 agent 已达到最大轮数。
var ErrMaxRounds = errors.New("agent runner reached max rounds")

// ErrMissingProviderDone 表示 provider 流结束时没有发送结束事件。
var ErrMissingProviderDone = errors.New("provider stream ended without done event")

// Runner 执行单个 agent 的 provider + tool loop。
type Runner struct {
	provider provider.Provider
	tools    *tool.Registry
	config   Config
}

// Config 保存 Runner 的运行配置。
type Config struct {
	Name         string
	SystemPrompt string
	ToolNames    []string
	MaxRounds    int
	Temperature  *float64
	MaxTokens    int
}

// Input 是一次 agent run 的输入。
type Input struct {
	UserMessage string
	Messages    []provider.Message
}

// Output 是一次 agent run 的输出。
type Output struct {
	FinalText string
	Messages  []provider.Message
	Rounds    int
}

// EventKind 表示 Runner 对外发送的事件类型。
type EventKind int

const (
	EventTextDelta EventKind = iota
	EventToolCall
	EventToolResult
	EventRoundDone
)

// Event 表示 Runner 运行过程中的结构化事件。
type Event struct {
	Kind     EventKind
	Text     string
	ToolCall *provider.ToolCall
	ToolName string
	IsError  bool
	Message  string
}

// EventSink 接收 Runner 运行事件。
type EventSink func(Event)

// NewRunner 创建 Runner 并校验配置。
func NewRunner(prov provider.Provider, registry *tool.Registry, cfg Config) (*Runner, error) {
	if prov == nil {
		return nil, fmt.Errorf("create agent runner: provider is nil")
	}
	if len(cfg.ToolNames) > 0 && registry == nil {
		return nil, fmt.Errorf("create agent runner: tool registry is nil")
	}
	if cfg.MaxRounds == 0 {
		cfg.MaxRounds = defaultMaxRounds
	}
	if cfg.MaxRounds < 1 || cfg.MaxRounds > 32 {
		return nil, fmt.Errorf("create agent runner: max rounds must be between 1 and 32")
	}

	return &Runner{
		provider: prov,
		tools:    registry,
		config:   cfg,
	}, nil
}

// Run 执行 agent，直到得到最终回答或遇到错误。
func (r *Runner) Run(ctx context.Context, input Input, sink EventSink) (Output, error) {
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}

	specs, err := r.toolSpecs()
	if err != nil {
		return Output{}, err
	}
	allowedTools := r.allowedToolSet()

	msgs := r.initialMessages(input)
	output := Output{Messages: append([]provider.Message(nil), msgs...)}

	for round := 1; round <= r.config.MaxRounds; round++ {
		if err := ctx.Err(); err != nil {
			output.Messages = msgs
			output.Rounds = round - 1
			return output, err
		}

		result, err := r.runRound(ctx, msgs, specs, sink, round)
		if err != nil {
			output.Messages = msgs
			output.Rounds = round
			return output, err
		}

		msgs = append(msgs, provider.Message{
			Role:      provider.RoleAssistant,
			Content:   result.assistantText,
			ToolCalls: result.toolCalls,
		})

		output.FinalText = result.assistantText
		output.Messages = append([]provider.Message(nil), msgs...)
		output.Rounds = round

		emit(sink, Event{Kind: EventRoundDone, Message: string(result.finish)})

		if result.finish != provider.FinishToolCalls || len(result.toolCalls) == 0 {
			return output, nil
		}

		for _, call := range result.toolCalls {
			msg, event, err := r.executeTool(ctx, call, allowedTools)
			if err != nil {
				output.Messages = msgs
				return output, err
			}
			msgs = append(msgs, msg)
			output.Messages = append([]provider.Message(nil), msgs...)
			emit(sink, event)
		}
	}

	output.Messages = append([]provider.Message(nil), msgs...)
	return output, fmt.Errorf("%w: agent %q max rounds %d", ErrMaxRounds, r.config.Name, r.config.MaxRounds)
}

type roundResult struct {
	assistantText string
	toolCalls     []provider.ToolCall
	finish        provider.FinishReason
}

func (r *Runner) runRound(ctx context.Context, msgs []provider.Message, specs []provider.ToolSpec, sink EventSink, round int) (roundResult, error) {
	ch, err := r.provider.Stream(ctx, provider.ChatRequest{
		Messages:    msgs,
		Tools:       specs,
		Temperature: r.config.Temperature,
		MaxTokens:   r.config.MaxTokens,
	})
	if err != nil {
		return roundResult{}, fmt.Errorf("agent %q round %d start provider %q stream: %w", r.config.Name, round, r.provider.Name(), err)
	}

	var result roundResult
	var sawDone bool
	var sawText bool

	for ev := range ch {
		switch ev.Kind {
		case provider.EventText:
			sawText = true
			result.assistantText += ev.TextDelta
			emit(sink, Event{Kind: EventTextDelta, Text: ev.TextDelta})
		case provider.EventToolCall:
			if ev.ToolCall == nil {
				return result, fmt.Errorf("agent %q round %d provider %q returned nil tool call", r.config.Name, round, r.provider.Name())
			}
			call := *ev.ToolCall
			result.toolCalls = append(result.toolCalls, call)
			emit(sink, Event{Kind: EventToolCall, ToolCall: &call, ToolName: call.Name})
		case provider.EventDone:
			sawDone = true
			result.finish = ev.FinishReason
		case provider.EventError:
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			return result, fmt.Errorf("agent %q round %d provider %q stream: %w", r.config.Name, round, r.provider.Name(), ev.Err)
		}
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	if !sawDone {
		if sawText && len(result.toolCalls) == 0 {
			result.finish = provider.FinishStop
			return result, nil
		}
		return result, fmt.Errorf("%w: agent %q round %d provider %q", ErrMissingProviderDone, r.config.Name, round, r.provider.Name())
	}

	return result, nil
}

func (r *Runner) executeTool(ctx context.Context, call provider.ToolCall, allowedTools map[string]bool) (provider.Message, Event, error) {
	if err := ctx.Err(); err != nil {
		return provider.Message{}, Event{}, err
	}

	content := ""
	isError := false
	if !allowedTools[call.Name] {
		content = fmt.Sprintf("tool %q is not available to this agent", call.Name)
		isError = true
	} else {
		t, ok := r.tools.Get(call.Name)
		if !ok {
			content = fmt.Sprintf("tool %q is not registered", call.Name)
			isError = true
		} else {
			result, err := t.Execute(ctx, []byte(call.Arguments))
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return provider.Message{}, Event{}, err
				}
				content = fmt.Sprintf("tool %q failed: %v", call.Name, err)
				isError = true
			} else {
				content = result.Content
				isError = result.IsError
			}
		}
	}

	msg := provider.Message{
		Role:       provider.RoleTool,
		ToolCallID: call.ID,
		Name:       call.Name,
		Content:    content,
	}
	event := Event{
		Kind:     EventToolResult,
		ToolName: call.Name,
		IsError:  isError,
		Message:  content,
	}
	return msg, event, nil
}

func (r *Runner) toolSpecs() ([]provider.ToolSpec, error) {
	if len(r.config.ToolNames) == 0 {
		return nil, nil
	}

	specs, err := r.tools.Specs(r.config.ToolNames)
	if err != nil {
		return nil, fmt.Errorf("agent %q load tool specs: %w", r.config.Name, err)
	}
	return specs, nil
}

func (r *Runner) allowedToolSet() map[string]bool {
	allowed := make(map[string]bool, len(r.config.ToolNames))
	for _, name := range r.config.ToolNames {
		allowed[name] = true
	}
	return allowed
}

func (r *Runner) initialMessages(input Input) []provider.Message {
	msgs := make([]provider.Message, 0, len(input.Messages)+2)
	if r.config.SystemPrompt != "" && !hasSystemMessage(input.Messages) {
		msgs = append(msgs, provider.Message{
			Role:    provider.RoleSystem,
			Content: r.config.SystemPrompt,
		})
	}
	msgs = append(msgs, input.Messages...)
	if input.UserMessage != "" {
		msgs = append(msgs, provider.Message{
			Role:    provider.RoleUser,
			Content: input.UserMessage,
		})
	}
	return msgs
}

func hasSystemMessage(messages []provider.Message) bool {
	for _, msg := range messages {
		if msg.Role == provider.RoleSystem {
			return true
		}
	}
	return false
}

func emit(sink EventSink, event Event) {
	if sink != nil {
		sink(event)
	}
}
