# 代码风格

本项目以 Google Go Style Guide 作为主要 Go 代码规范。

参考资料：

- Google Go Style Guide: https://google.github.io/styleguide/go/guide
- Google Go Style Decisions: https://google.github.io/styleguide/go/decisions
- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments

## 核心规则

- 所有 Go 代码必须使用 `gofmt` 格式化。
- 代码优先级依次为：清晰、简单、简洁、可维护、一致。
- 使用符合 Go 习惯的命名和包结构。
- 接口要小，并尽量靠近使用方定义。
- 返回错误时要带有有用上下文，不要静默忽略错误。
- 除不可恢复的 CLI 启动失败外，避免使用 `panic`。

## 命名

Go 标识符使用 `MixedCaps` 或 `mixedCaps`，不要使用 `snake_case`。

示例：

```go
type ProviderConfig struct {
	BaseURL   string
	APIKeyEnv string
}

var taskID string
var runID string
var createdAt time.Time
```

常见缩写保持统一大写：

- `ID`，不是 `Id`
- `URL`，不是 `Url`
- `API`，不是 `Api`
- `HTTP`，不是 `Http`
- `JSON`，不是 `Json`
- `SQL`，不是 `Sql`

示例：

```go
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func loadAPIKey(apiKeyEnv string) (string, error) {
	// ...
}
```

## 项目约定

不同层使用不同命名风格：

| 区域 | 约定 | 示例 |
| --- | --- | --- |
| Go 标识符 | `mixedCaps` / `MixedCaps` | `taskID`, `ProviderConfig` |
| Go 文件名 | `snake_case` | `tool_executor.go` |
| 包名 | 短小写单词 | `agent`, `tool`, `provider` |
| YAML 字段 | `snake_case` | `api_key_env` |
| JSON 字段 | `snake_case` | `run_id` |
| SQLite 字段 | `snake_case` | `created_at` |
| CLI flag | `kebab-case` | `--config-file` |
| 环境变量 | `UPPER_SNAKE_CASE` | `DEEPSEEK_API_KEY` |

映射外部字段到 Go 结构体时，内部使用 Go 命名，外部用 tag：

```go
type ProviderConfig struct {
	BaseURL   string `yaml:"base_url" json:"base_url"`
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`
}
```

## 错误处理

错误要立刻处理，并保持正常路径少缩进：

```go
value, err := loadValue()
if err != nil {
	return fmt.Errorf("load value: %w", err)
}

return useValue(value)
```

错误字符串使用小写开头，不要以标点结尾，除非消息本身必须包含。

## Context

- 可能阻塞、调用模型、执行工具或访问存储的操作，第一个参数必须是 `context.Context`。
- 不要把 `context.Context` 存进结构体。
- 必须尊重取消和 deadline。

示例：

```go
func (r *Runner) Run(ctx context.Context, input Input) (Output, error) {
	// ...
}
```

## 注释

- 项目文档和代码注释必须使用中文。
- 对外导出的包、类型、函数、方法和常量，如果属于公共 API，应写文档注释。
- 文档注释应尽量是完整句子，并在可读性允许时以被注释的标识符开头。
- 避免写只重复代码含义的注释。

示例：

```go
// Runner 执行一个 agent，直到它返回最终回答或失败。
type Runner struct {
	// ...
}
```
