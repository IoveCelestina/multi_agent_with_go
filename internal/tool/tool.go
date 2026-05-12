package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/ht/multi_agent/internal/provider"
)

var toolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Tool is a Go-implemented capability that an agent can call.
type Tool interface {
	Name() string
	Description() string
	JSONSchema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (Result, error)
}

// Result is the text returned to the model after a tool call.
type Result struct {
	Content string
	IsError bool
}

// Registry stores all available tools by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register adds a tool to the registry.
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

// Get returns a registered tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Specs returns provider tool specs for the named tools.
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
