package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeTool struct {
	name        string
	description string
	schema      json.RawMessage
}

func (f fakeTool) Name() string {
	return f.name
}

func (f fakeTool) Description() string {
	return f.description
}

func (f fakeTool) JSONSchema() json.RawMessage {
	return f.schema
}

func (f fakeTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	return Result{Content: "ok"}, nil
}

func TestRegistryRegisterAndSpecs(t *testing.T) {
	registry := NewRegistry()
	tool := fakeTool{
		name:        "read_file",
		description: "read a file",
		schema:      json.RawMessage(`{"type":"object"}`),
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	specs, err := registry.Specs([]string{"read_file"})
	if err != nil {
		t.Fatalf("Specs() error = %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1", len(specs))
	}
	if specs[0].Name != "read_file" {
		t.Fatalf("spec name = %q, want read_file", specs[0].Name)
	}
}

func TestRegistryRejectsInvalidTool(t *testing.T) {
	tests := []struct {
		name string
		tool fakeTool
		want string
	}{
		{
			name: "invalid name",
			tool: fakeTool{name: "ReadFile", description: "read", schema: json.RawMessage(`{}`)},
			want: "name must match",
		},
		{
			name: "empty description",
			tool: fakeTool{name: "read_file", schema: json.RawMessage(`{}`)},
			want: "description is empty",
		},
		{
			name: "invalid schema",
			tool: fakeTool{name: "read_file", description: "read", schema: json.RawMessage(`{`)},
			want: "json schema is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRegistry().Register(tt.tool)
			if err == nil {
				t.Fatal("Register() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Register() error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestRegistryRejectsDuplicateAndUnknownSpecs(t *testing.T) {
	registry := NewRegistry()
	tool := fakeTool{
		name:        "read_file",
		description: "read",
		schema:      json.RawMessage(`{}`),
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.Register(tool); err == nil {
		t.Fatal("Register() duplicate error = nil")
	}

	if _, err := registry.Specs([]string{"missing"}); err == nil {
		t.Fatal("Specs() unknown tool error = nil")
	}
}
