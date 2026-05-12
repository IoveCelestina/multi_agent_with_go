# Coding Style

This project follows the Google Go Style Guide as the primary Go coding standard.

References:

- Google Go Style Guide: https://google.github.io/styleguide/go/guide
- Google Go Style Decisions: https://google.github.io/styleguide/go/decisions
- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments

## Core Rules

- Format all Go code with `gofmt`.
- Prefer clarity, simplicity, concision, maintainability, and consistency, in that order.
- Use idiomatic Go names and package structure.
- Keep interfaces small and close to the code that consumes them.
- Return errors with useful context; do not silently ignore errors.
- Avoid `panic` outside unrecoverable CLI startup failures.

## Naming

Go identifiers use `MixedCaps` or `mixedCaps`, not `snake_case`.

Examples:

```go
type ProviderConfig struct {
	BaseURL   string
	APIKeyEnv string
}

var taskID string
var runID string
var createdAt time.Time
```

Use consistent capitalization for initialisms:

- `ID`, not `Id`
- `URL`, not `Url`
- `API`, not `Api`
- `HTTP`, not `Http`
- `JSON`, not `Json`
- `SQL`, not `Sql`

Examples:

```go
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func loadAPIKey(apiKeyEnv string) (string, error) {
	// ...
}
```

## Project Conventions

Different layers use different naming conventions:

| Area | Convention | Example |
| --- | --- | --- |
| Go identifiers | `mixedCaps` / `MixedCaps` | `taskID`, `ProviderConfig` |
| Go filenames | `snake_case` | `tool_executor.go` |
| Package names | short lowercase words | `agent`, `tool`, `provider` |
| YAML fields | `snake_case` | `api_key_env` |
| JSON fields | `snake_case` | `run_id` |
| SQLite columns | `snake_case` | `created_at` |
| CLI flags | `kebab-case` | `--config-file` |
| Environment variables | `UPPER_SNAKE_CASE` | `DEEPSEEK_API_KEY` |

When mapping external fields to Go structs, use Go names internally and tags externally:

```go
type ProviderConfig struct {
	BaseURL   string `yaml:"base_url" json:"base_url"`
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`
}
```

## Errors

Handle errors immediately and keep the normal path unindented:

```go
value, err := loadValue()
if err != nil {
	return fmt.Errorf("load value: %w", err)
}

return useValue(value)
```

Error strings should be lower-case and should not end with punctuation unless required by the message.

## Context

- Pass `context.Context` as the first parameter for operations that can block, call models, execute tools, or touch storage.
- Do not store `context.Context` in structs.
- Respect cancellation and deadlines.

Example:

```go
func (r *Runner) Run(ctx context.Context, input Input) (Output, error) {
	// ...
}
```

## Comments

- Project documentation and code comments must be written in Chinese.
- Exported packages, types, functions, methods, and constants should have doc comments when they are part of a public API.
- Doc comments should be complete sentences and start with the documented identifier when practical.
- Avoid comments that merely repeat the code.

Example:

```go
// Runner executes an agent until it returns a final response or fails.
type Runner struct {
	// ...
}
```
