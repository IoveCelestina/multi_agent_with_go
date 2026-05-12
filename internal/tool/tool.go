package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/ht/multi_agent/internal/provider"
)

var toolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// 可供 agent 调用的 Go 实现能力。
type Tool interface {
	Name() string
	Description() string
	JSONSchema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (Result, error)
}

// 工具调用后回传给模型的结果。
type Result struct {
	Content string
	IsError bool
}

// 按名称保存所有可用工具。
type Registry struct {
	tools map[string]Tool
}

// 创建一个空工具注册表。
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// 把工具加入注册表。
func (r *Registry) Register(t Tool) error {
	if t == nil {
		return fmt.Errorf("register tool: tool is nil")
	}

	name := t.Name()
	if name == "" {
		return fmt.Errorf("register tool: name is empty")
	}
	if !toolNamePattern.MatchString(name) {
		return fmt.Errorf("register tool %q: name must match %s", name, toolNamePattern.String())
	}
	if t.Description() == "" {
		return fmt.Errorf("register tool %q: description is empty", name)
	}
	if !json.Valid(t.JSONSchema()) {
		return fmt.Errorf("register tool %q: json schema is invalid", name)
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("register tool %q: already registered", name)
	}

	r.tools[name] = t
	return nil
}

// 按名称返回已注册工具。
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// 返回指定工具名对应的 provider 工具声明。
func (r *Registry) Specs(names []string) ([]provider.ToolSpec, error) {
	specs := make([]provider.ToolSpec, 0, len(names))
	for _, name := range names {
		t, ok := r.Get(name)
		if !ok {
			return nil, fmt.Errorf("tool %q is not registered", name)
		}

		specs = append(specs, provider.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			JSONSchema:  t.JSONSchema(),
		})
	}

	return specs, nil
}
