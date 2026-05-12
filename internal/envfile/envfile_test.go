package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSetsValuesAndDoesNotOverrideExistingEnv(t *testing.T) {
	t.Setenv("EXISTING_KEY", "from-env")
	t.Setenv("PLAIN_KEY", "")
	_ = os.Unsetenv("QUOTED_KEY")

	path := writeEnvFile(t, `# comment
PLAIN_KEY=from-file
QUOTED_KEY="hello world"
EXISTING_KEY=from-file
export EXPORTED_KEY='ok'
`)

	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := os.Getenv("PLAIN_KEY"); got != "" {
		t.Fatalf("PLAIN_KEY = %q, want existing empty value", got)
	}
	if got := os.Getenv("QUOTED_KEY"); got != "hello world" {
		t.Fatalf("QUOTED_KEY = %q, want hello world", got)
	}
	if got := os.Getenv("EXISTING_KEY"); got != "from-env" {
		t.Fatalf("EXISTING_KEY = %q, want from-env", got)
	}
	if got := os.Getenv("EXPORTED_KEY"); got != "ok" {
		t.Fatalf("EXPORTED_KEY = %q, want ok", got)
	}
}

func TestLoadMissingFileIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsInvalidLine(t *testing.T) {
	path := writeEnvFile(t, "INVALID\n")
	if err := Load(path); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
