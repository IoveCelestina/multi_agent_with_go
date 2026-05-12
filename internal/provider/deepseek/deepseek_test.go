package deepseek

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ht/multi_agent/internal/config"
	"github.com/ht/multi_agent/internal/provider"
)

func TestStreamEmitsTextAndDone(t *testing.T) {
	t.Setenv("DEEPSEEK_TEST_KEY", "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if req.Model != "deepseek-chat" {
			t.Fatalf("model = %q, want deepseek-chat", req.Model)
		}
		if !req.Stream {
			t.Fatal("stream = false")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	prov := newTestProvider(t, server.URL)
	events, err := prov.Stream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var text string
	var finish provider.FinishReason
	for event := range events {
		switch event.Kind {
		case provider.EventText:
			text += event.TextDelta
		case provider.EventDone:
			finish = event.FinishReason
		case provider.EventError:
			t.Fatalf("EventError = %v", event.Err)
		}
	}

	if text != "hello world" {
		t.Fatalf("text = %q, want hello world", text)
	}
	if finish != provider.FinishStop {
		t.Fatalf("finish = %q, want %q", finish, provider.FinishStop)
	}
}

func TestStreamAssemblesToolCall(t *testing.T) {
	t.Setenv("DEEPSEEK_TEST_KEY", "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"read_file\",\"arguments\":\"{\\\"path\\\":\"}}]}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"README.md\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	prov := newTestProvider(t, server.URL)
	events, err := prov.Stream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "read"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var calls []provider.ToolCall
	var finish provider.FinishReason
	for event := range events {
		switch event.Kind {
		case provider.EventToolCall:
			calls = append(calls, *event.ToolCall)
		case provider.EventDone:
			finish = event.FinishReason
		case provider.EventError:
			t.Fatalf("EventError = %v", event.Err)
		}
	}

	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].ID != "call_1" {
		t.Fatalf("call ID = %q, want call_1", calls[0].ID)
	}
	if calls[0].Name != "read_file" {
		t.Fatalf("call name = %q, want read_file", calls[0].Name)
	}
	if calls[0].Arguments != `{"path":"README.md"}` {
		t.Fatalf("arguments = %q", calls[0].Arguments)
	}
	if finish != provider.FinishToolCalls {
		t.Fatalf("finish = %q, want %q", finish, provider.FinishToolCalls)
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	const envName = "DEEPSEEK_MISSING_KEY"
	_ = os.Unsetenv(envName)

	if _, err := New(config.ProviderConfig{
		BaseURL:   "https://api.deepseek.com",
		Model:     "deepseek-chat",
		APIKeyEnv: envName,
	}, nil); err == nil {
		t.Fatal("New() error = nil")
	}
}

func newTestProvider(t *testing.T, baseURL string) *Provider {
	t.Helper()

	prov, err := New(config.ProviderConfig{
		BaseURL:   baseURL,
		Model:     "deepseek-chat",
		APIKeyEnv: "DEEPSEEK_TEST_KEY",
	}, http.DefaultClient)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return prov
}
