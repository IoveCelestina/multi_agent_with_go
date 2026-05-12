package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/ht/multi_agent/internal/config"
	"github.com/ht/multi_agent/internal/envfile"
	"github.com/ht/multi_agent/internal/provider"
	"github.com/ht/multi_agent/internal/provider/deepseek"
	"github.com/ht/multi_agent/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version.String())
	case "chat":
		if err := runChat(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "chat failed: %v\n", err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println(`agentctl is a CLI for running multi-agent workflows.

Usage:
  agentctl chat [--provider deepseek] [--config configs/agents.yaml] <prompt>
  agentctl version
  agentctl help

Environment:
  .env is loaded automatically when present.
  DEEPSEEK_API_KEY is required when using --provider deepseek`)
}

func runChat(args []string) error {
	flags := flag.NewFlagSet("chat", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	configPath := flags.String("config", "configs/agents.yaml", "path to config file")
	providerName := flags.String("provider", "", "provider name")
	temperature := flags.Float64("temperature", -1, "sampling temperature")
	maxTokens := flags.Int("max-tokens", 0, "maximum output tokens")

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse chat flags: %w", err)
	}

	prompt := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	if err := envfile.Load(".env"); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	name := *providerName
	if name == "" {
		name = cfg.Providers.Default
	}

	prov, err := newProvider(name, cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var requestTemperature *float64
	if *temperature >= 0 {
		requestTemperature = temperature
	}

	events, err := prov.Stream(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: prompt},
		},
		Temperature: requestTemperature,
		MaxTokens:   *maxTokens,
	})
	if err != nil {
		return err
	}

	var wroteText bool
	for event := range events {
		switch event.Kind {
		case provider.EventText:
			wroteText = true
			fmt.Print(event.TextDelta)
		case provider.EventToolCall:
			return fmt.Errorf("provider returned tool call %q, but chat command does not execute tools yet", event.ToolCall.Name)
		case provider.EventDone:
			if wroteText {
				fmt.Println()
			}
			return nil
		case provider.EventError:
			if errors.Is(event.Err, context.Canceled) {
				if wroteText {
					fmt.Println()
				}
				return nil
			}
			return event.Err
		}
	}

	if wroteText {
		fmt.Println()
	}
	return nil
}

func newProvider(name string, cfg config.Config) (provider.Provider, error) {
	providerConfig, ok := cfg.Providers.Items[name]
	if !ok {
		return nil, fmt.Errorf("provider %q is not configured", name)
	}

	switch name {
	case "deepseek":
		return deepseek.New(providerConfig, http.DefaultClient)
	default:
		return nil, fmt.Errorf("provider %q is not supported yet", name)
	}
}
