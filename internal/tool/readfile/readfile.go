package readfile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ht/multi_agent/internal/tool"
)

const (
	defaultMaxBytes = int64(64 * 1024)
)

var schema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "absolute or relative path"
    },
    "offset": {
      "type": "integer",
      "minimum": 0
    },
    "limit": {
      "type": "integer",
      "minimum": 1
    }
  },
  "required": ["path"]
}`)

// 从显式允许的 roots 中读取文件。
type Tool struct {
	Roots    []string
	MaxBytes int64

	roots []string
}

type arguments struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Limit  *int64 `json:"limit"`
}

// 创建 read_file 工具，并校验它的沙箱 roots。
func New(roots []string, maxBytes int64) (*Tool, error) {
	t := &Tool{
		Roots:    append([]string(nil), roots...),
		MaxBytes: maxBytes,
	}
	if err := t.init(); err != nil {
		return nil, err
	}

	return t, nil
}

// 返回暴露给模型的工具名。
func (t *Tool) Name() string {
	return "read_file"
}

// 返回暴露给模型的工具描述。
func (t *Tool) Description() string {
	return "Read a UTF-8 text file from an allowed workspace path."
}

// 返回给模型看的参数 schema。
func (t *Tool) JSONSchema() json.RawMessage {
	return schema
}

// 读取请求的文件片段。
func (t *Tool) Execute(ctx context.Context, args json.RawMessage) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}

	if len(t.roots) == 0 {
		if err := t.init(); err != nil {
			return tool.Result{}, err
		}
	}

	var parsed arguments
	if err := json.Unmarshal(args, &parsed); err != nil {
		return errorResult("invalid read_file arguments: %v", err), nil
	}
	if strings.TrimSpace(parsed.Path) == "" {
		return errorResult("invalid read_file arguments: path is required"), nil
	}
	if parsed.Offset < 0 {
		return errorResult("invalid read_file arguments: offset must be greater than or equal to 0"), nil
	}

	limit := t.maxBytes()
	if parsed.Limit != nil {
		if *parsed.Limit < 1 {
			return errorResult("invalid read_file arguments: limit must be greater than 0"), nil
		}
		if *parsed.Limit > t.maxBytes() {
			return errorResult("invalid read_file arguments: limit exceeds max_bytes %d", t.maxBytes()), nil
		}
		limit = *parsed.Limit
	}

	path, err := resolveUserPath(parsed.Path)
	if err != nil {
		return errorResult("read_file path is not accessible: %v", err), nil
	}

	if !t.allowed(path) {
		return errorResult("read_file path is outside allowed roots: %s", parsed.Path), nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return errorResult("read_file stat failed: %v", err), nil
	}
	if info.IsDir() {
		return errorResult("read_file path is a directory: %s", parsed.Path), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return errorResult("read_file open failed: %v", err), nil
	}
	defer file.Close()

	if _, err := file.Seek(parsed.Offset, io.SeekStart); err != nil {
		return errorResult("read_file seek failed: %v", err), nil
	}

	buf, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return errorResult("read_file read failed: %v", err), nil
	}

	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}

	return tool.Result{Content: string(buf)}, nil
}

func (t *Tool) init() error {
	if len(t.Roots) == 0 {
		return fmt.Errorf("initialize read_file: roots is empty")
	}
	roots := make([]string, 0, len(t.Roots))
	for _, root := range t.Roots {
		if strings.TrimSpace(root) == "" {
			return fmt.Errorf("initialize read_file: root is empty")
		}

		abs, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("initialize read_file root %q: %w", root, err)
		}

		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return fmt.Errorf("initialize read_file root %q: %w", root, err)
		}

		info, err := os.Stat(resolved)
		if err != nil {
			return fmt.Errorf("initialize read_file root %q: %w", root, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("initialize read_file root %q: not a directory", root)
		}

		roots = append(roots, filepath.Clean(resolved))
	}

	t.roots = roots
	return nil
}

func (t *Tool) maxBytes() int64 {
	if t.MaxBytes <= 0 {
		return defaultMaxBytes
	}

	return t.MaxBytes
}

func (t *Tool) allowed(path string) bool {
	for _, root := range t.roots {
		if pathInRoot(path, root) {
			return true
		}
	}

	return false
}

func resolveUserPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}

	return filepath.Clean(resolved), nil
}

func pathInRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)

	if samePath(path, root) {
		return true
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || filepath.IsAbs(rel) {
		return false
	}

	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

func errorResult(format string, args ...any) tool.Result {
	return tool.Result{
		Content: fmt.Sprintf(format, args...),
		IsError: true,
	}
}
