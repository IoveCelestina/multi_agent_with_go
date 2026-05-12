package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsProvidersFromProjectConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "configs", "agents.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Version != 1 {
		t.Fatalf("Version = %d, want 1", cfg.Version)
	}
	if cfg.Runtime.MaxRounds != 10 {
		t.Fatalf("MaxRounds = %d, want 10", cfg.Runtime.MaxRounds)
	}
	if cfg.Providers.Default != "deepseek" {
		t.Fatalf("Default provider = %q, want deepseek", cfg.Providers.Default)
	}

	deepseek := cfg.Providers.Items["deepseek"]
	if deepseek.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("DeepSeek BaseURL = %q", deepseek.BaseURL)
	}
	if deepseek.Model == "" {
		t.Fatal("DeepSeek Model is empty")
	}
	if deepseek.APIKeyEnv != "DEEPSEEK_API_KEY" {
		t.Fatalf("DeepSeek APIKeyEnv = %q", deepseek.APIKeyEnv)
	}

	researcher := cfg.Agents["researcher"]
	if researcher.Provider != "deepseek" {
		t.Fatalf("Researcher Provider = %q, want deepseek", researcher.Provider)
	}
	if researcher.SystemPrompt == "" {
		t.Fatal("Researcher SystemPrompt is empty")
	}
	if len(researcher.Tools) != 1 || researcher.Tools[0] != "read_file" {
		t.Fatalf("Researcher Tools = %#v, want [read_file]", researcher.Tools)
	}

	readFile := cfg.Tools["read_file"]
	if len(readFile.Roots) != 1 || readFile.Roots[0] != "./" {
		t.Fatalf("read_file roots = %#v, want [./]", readFile.Roots)
	}
	if readFile.MaxBytes != 65536 {
		t.Fatalf("read_file max_bytes = %d, want 65536", readFile.MaxBytes)
	}

	workflow := cfg.Workflows["research_writer"]
	if workflow.Coordinator != "sync" {
		t.Fatalf("workflow coordinator = %q, want sync", workflow.Coordinator)
	}
	if len(workflow.Steps) != 2 {
		t.Fatalf("workflow steps = %d, want 2", len(workflow.Steps))
	}
	if workflow.Steps[1].Input != "research_notes" {
		t.Fatalf("second step input = %q, want research_notes", workflow.Steps[1].Input)
	}
}

func TestLoadRejectsInvalidMaxRounds(t *testing.T) {
	path := writeConfig(t, `version: 1

runtime:
  max_rounds: 0

providers:
  default: deepseek
  deepseek:
    base_url: https://api.deepseek.com
    model: deepseek-chat
    api_key_env: DEEPSEEK_API_KEY
`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "agents.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
