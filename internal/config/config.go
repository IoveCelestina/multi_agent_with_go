package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// This phase-1 loader intentionally parses only version, runtime, and providers.
// Agents, tools, and workflows stay in the YAML file but are parsed in later phases.

// Config is the project runtime configuration.
type Config struct {
	Version   int
	Runtime   RuntimeConfig
	Providers ProvidersConfig
}

// RuntimeConfig contains agent runtime defaults.
type RuntimeConfig struct {
	MaxRounds int
}

// ProvidersConfig contains model provider settings.
type ProvidersConfig struct {
	Default string
	Items   map[string]ProviderConfig
}

// ProviderConfig contains one provider's endpoint and model settings.
type ProviderConfig struct {
	BaseURL   string
	Model     string
	APIKeyEnv string
}

// Load reads the small project YAML config.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	cfg := Config{
		Runtime: RuntimeConfig{MaxRounds: 10},
		Providers: ProvidersConfig{
			Items: map[string]ProviderConfig{},
		},
	}

	var section string
	var currentProvider string

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		raw := strings.TrimRight(scanner.Text(), " \t")
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		indent := countLeadingSpaces(raw)
		if !strings.Contains(line, ":") && indent > 0 {
			continue
		}

		key, value, ok := splitYAMLScalar(line)
		if !ok {
			return Config{}, fmt.Errorf("parse config line %d: expected key: value", lineNumber)
		}

		switch indent {
		case 0:
			currentProvider = ""
			if value == "" {
				section = key
				continue
			}

			section = ""
			switch key {
			case "version":
				version, err := strconv.Atoi(value)
				if err != nil {
					return Config{}, fmt.Errorf("parse config line %d: version must be an integer", lineNumber)
				}
				cfg.Version = version
			default:
				continue
			}
		case 2:
			switch section {
			case "runtime":
				if key == "max_rounds" {
					maxRounds, err := strconv.Atoi(value)
					if err != nil {
						return Config{}, fmt.Errorf("parse config line %d: max_rounds must be an integer", lineNumber)
					}
					cfg.Runtime.MaxRounds = maxRounds
				}
			case "providers":
				if key == "default" {
					cfg.Providers.Default = value
					currentProvider = ""
				} else {
					currentProvider = key
					if _, ok := cfg.Providers.Items[currentProvider]; !ok {
						cfg.Providers.Items[currentProvider] = ProviderConfig{}
					}
				}
			}
		case 4:
			if section != "providers" || currentProvider == "" {
				continue
			}

			provider := cfg.Providers.Items[currentProvider]
			switch key {
			case "base_url":
				provider.BaseURL = value
			case "model":
				provider.Model = value
			case "api_key_env":
				provider.APIKeyEnv = value
			}
			cfg.Providers.Items[currentProvider] = provider
		default:
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
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

func countLeadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		count++
	}

	return count
}

func splitYAMLScalar(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return "", "", false
	}

	return key, strings.Trim(value, `"`), true
}
