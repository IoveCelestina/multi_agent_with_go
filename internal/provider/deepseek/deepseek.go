package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ht/multi_agent/internal/config"
	"github.com/ht/multi_agent/internal/provider"
)

const defaultResponseHeaderTimeout = 30 * time.Second

// 实现 DeepSeek 的 OpenAI-compatible chat completions API。
type Provider struct {
	name       string
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

// 创建 DeepSeek provider。
func New(cfg config.ProviderConfig, client *http.Client) (*Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("create deepseek provider: base_url is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("create deepseek provider: model is required")
	}
	if cfg.APIKeyEnv == "" {
		return nil, fmt.Errorf("create deepseek provider: api_key_env is required")
	}

	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("create deepseek provider: environment variable %s is not set", cfg.APIKeyEnv)
	}
	if client == nil {
		client = defaultHTTPClient()
	}

	return &Provider{
		name:       "deepseek",
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		model:      cfg.Model,
		apiKey:     apiKey,
		httpClient: client,
	}, nil
}

// 返回 provider 名称。
func (p *Provider) Name() string {
	return p.name
}

// 启动一次流式 chat completion 请求。
func (p *Provider) Stream(ctx context.Context, req provider.ChatRequest) (<-chan provider.Event, error) {
	body, err := p.buildRequestBody(req)
	if err != nil {
		return nil, err
	}

	endpoint, err := chatCompletionsURL(p.baseURL)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create deepseek request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("start deepseek stream: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("start deepseek stream: status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	events := make(chan provider.Event)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		p.readStream(ctx, resp.Body, events)
	}()

	return events, nil
}

func (p *Provider) buildRequestBody(req provider.ChatRequest) ([]byte, error) {
	wire := chatRequest{
		Model:    p.model,
		Messages: make([]message, 0, len(req.Messages)),
		Stream:   true,
	}
	if req.Temperature != nil {
		wire.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		wire.MaxTokens = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		wire.Tools = make([]toolSpec, 0, len(req.Tools))
		for _, t := range req.Tools {
			wire.Tools = append(wire.Tools, toolSpec{
				Type: "function",
				Function: functionSpec{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.JSONSchema,
				},
			})
		}
	}

	for _, msg := range req.Messages {
		wire.Messages = append(wire.Messages, convertMessage(msg))
	}

	body, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("marshal deepseek request: %w", err)
	}

	return body, nil
}

func (p *Provider) readStream(ctx context.Context, body io.Reader, events chan<- provider.Event) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var assembler toolCallAssembler
	doneSent := false
	sendDone := func(reason provider.FinishReason) bool {
		if doneSent {
			return true
		}
		for _, call := range assembler.flush() {
			if !sendEvent(ctx, events, provider.Event{Kind: provider.EventToolCall, ToolCall: &call}) {
				return false
			}
		}
		if !sendEvent(ctx, events, provider.Event{
			Kind:         provider.EventDone,
			FinishReason: reason,
		}) {
			return false
		}
		doneSent = true
		return true
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			if !doneSent {
				_ = sendDone(provider.FinishStop)
			}
			return
		}

		var chunk chatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			sendEvent(ctx, events, provider.Event{Kind: provider.EventError, Err: fmt.Errorf("decode deepseek stream: %w", err)})
			return
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				if !sendEvent(ctx, events, provider.Event{Kind: provider.EventText, TextDelta: choice.Delta.Content}) {
					return
				}
			}

			for _, delta := range choice.Delta.ToolCalls {
				assembler.add(delta)
			}

			if choice.FinishReason != "" {
				if !sendDone(provider.FinishReason(choice.FinishReason)) {
					return
				}
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return
		}
		sendEvent(ctx, events, provider.Event{Kind: provider.EventError, Err: fmt.Errorf("read deepseek stream: %w", err)})
	}
}

func sendEvent(ctx context.Context, events chan<- provider.Event, event provider.Event) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}

func chatCompletionsURL(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse deepseek base_url: %w", err)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	return parsed.String(), nil
}

func convertMessage(msg provider.Message) message {
	converted := message{
		Role:       string(msg.Role),
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
		Name:       msg.Name,
	}
	if len(msg.ToolCalls) > 0 {
		converted.ToolCalls = make([]toolCall, 0, len(msg.ToolCalls))
		for _, call := range msg.ToolCalls {
			converted.ToolCalls = append(converted.ToolCalls, toolCall{
				ID:   call.ID,
				Type: "function",
				Function: functionCall{
					Name:      call.Name,
					Arguments: call.Arguments,
				},
			})
		}
	}

	return converted
}

func defaultHTTPClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{
			Transport: &http.Transport{ResponseHeaderTimeout: defaultResponseHeaderTimeout},
		}
	}

	cloned := transport.Clone()
	cloned.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	return &http.Client{Transport: cloned}
}

type chatRequest struct {
	Model       string     `json:"model"`
	Messages    []message  `json:"messages"`
	Tools       []toolSpec `json:"tools,omitempty"`
	Stream      bool       `json:"stream"`
	Temperature *float64   `json:"temperature,omitempty"`
	MaxTokens   int        `json:"max_tokens,omitempty"`
}

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type toolSpec struct {
	Type     string       `json:"type"`
	Function functionSpec `json:"function"`
}

type functionSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type chatChunk struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Delta        delta  `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type delta struct {
	Content   string          `json:"content"`
	ToolCalls []toolCallDelta `json:"tool_calls"`
}

type toolCallDelta struct {
	Index    int          `json:"index"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type toolCallAssembler struct {
	calls map[int]*provider.ToolCall
}

func (a *toolCallAssembler) add(delta toolCallDelta) {
	if a.calls == nil {
		a.calls = map[int]*provider.ToolCall{}
	}

	call, ok := a.calls[delta.Index]
	if !ok {
		call = &provider.ToolCall{}
		a.calls[delta.Index] = call
	}
	if delta.ID != "" {
		call.ID = delta.ID
	}
	if delta.Function.Name != "" {
		call.Name += delta.Function.Name
	}
	if delta.Function.Arguments != "" {
		call.Arguments += delta.Function.Arguments
	}
}

func (a *toolCallAssembler) flush() []provider.ToolCall {
	if len(a.calls) == 0 {
		return nil
	}

	indices := make([]int, 0, len(a.calls))
	for i := range a.calls {
		indices = append(indices, i)
	}
	sort.Ints(indices)

	out := make([]provider.ToolCall, 0, len(indices))
	for _, i := range indices {
		out = append(out, *a.calls[i])
	}
	a.calls = nil

	return out
}
