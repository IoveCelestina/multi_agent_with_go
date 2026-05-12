package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// 表示项目运行配置。
type Config struct {
	Version   int                       `yaml:"version"`
	Runtime   RuntimeConfig             `yaml:"runtime"`
	Providers ProvidersConfig           `yaml:"providers"`
	Agents    map[string]AgentConfig    `yaml:"agents"`
	Tools     map[string]ToolConfig     `yaml:"tools"`
	Workflows map[string]WorkflowConfig `yaml:"workflows"`
}

// 保存 agent 运行时默认配置。
type RuntimeConfig struct {
	MaxRounds int `yaml:"max_rounds"`
}

// 保存模型 provider 配置。
type ProvidersConfig struct {
	Default string                    `yaml:"default"`
	Items   map[string]ProviderConfig `yaml:",inline"`
}

// 保存一个 provider 的 endpoint 和模型配置。
type ProviderConfig struct {
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
	APIKeyEnv string `yaml:"api_key_env"`
}

// 保存一个 agent 的模型和工具配置。
type AgentConfig struct {
	Provider     string   `yaml:"provider"`
	SystemPrompt string   `yaml:"system_prompt"`
	Tools        []string `yaml:"tools"`
	MaxRounds    int      `yaml:"max_rounds"`
}

// 保存工具专属运行配置。
type ToolConfig struct {
	Roots    []string `yaml:"roots"`
	MaxBytes int64    `yaml:"max_bytes"`
}

// 保存一个 workflow 定义。
type WorkflowConfig struct {
	Coordinator string         `yaml:"coordinator"`
	Steps       []WorkflowStep `yaml:"steps"`
}

// 保存一个 workflow step。
type WorkflowStep struct {
	Agent  string `yaml:"agent"`
	Input  string `yaml:"input"`
	Output string `yaml:"output"`
}

// 读取项目 YAML 配置。
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	cfg := raw.toConfig()
	if cfg.Providers.Items == nil {
		cfg.Providers.Items = map[string]ProviderConfig{}
	}

	if cfg.Version == 0 {
		return Config{}, fmt.Errorf("config version is required")
	}
	if cfg.Providers.Default == "" {
		return Config{}, fmt.Errorf("providers.default is required")
	}
	if _, ok := cfg.Providers.Items[cfg.Providers.Default]; !ok {
		return Config{}, fmt.Errorf("default provider %q is not configured", cfg.Providers.Default)
	}
	for name, provider := range cfg.Providers.Items {
		if provider.BaseURL == "" {
			return Config{}, fmt.Errorf("provider %q base_url is required", name)
		}
		if provider.Model == "" {
			return Config{}, fmt.Errorf("provider %q model is required", name)
		}
		if provider.APIKeyEnv == "" {
			return Config{}, fmt.Errorf("provider %q api_key_env is required", name)
		}
	}
	if cfg.Runtime.MaxRounds < 1 || cfg.Runtime.MaxRounds > 32 {
		return Config{}, fmt.Errorf("runtime.max_rounds must be between 1 and 32")
	}

	return cfg, nil
}

type rawConfig struct {
	Version  int                       `yaml:"version"`
	Runtime  rawRuntimeConfig          `yaml:"runtime"`
	Provider ProvidersConfig           `yaml:"providers"`
	Agents   map[string]AgentConfig    `yaml:"agents"`
	Tools    map[string]ToolConfig     `yaml:"tools"`
	Workflow map[string]WorkflowConfig `yaml:"workflows"`
}

type rawRuntimeConfig struct {
	MaxRounds *int `yaml:"max_rounds"`
}

func (r rawConfig) toConfig() Config {
	maxRounds := 10
	if r.Runtime.MaxRounds != nil {
		maxRounds = *r.Runtime.MaxRounds
	}

	return Config{
		Version:   r.Version,
		Runtime:   RuntimeConfig{MaxRounds: maxRounds},
		Providers: r.Provider,
		Agents:    r.Agents,
		Tools:     r.Tools,
		Workflows: r.Workflow,
	}
}
