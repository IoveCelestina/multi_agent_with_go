package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Load reads environment variables from path without overriding existing values.
func Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("parse env file line %d: expected KEY=value", lineNumber)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("parse env file line %d: key is empty", lineNumber)
		}
		if !validKey(key) {
			return fmt.Errorf("parse env file line %d: invalid key %q", lineNumber, key)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.TrimSpace(value)
		value, err = unquote(value)
		if err != nil {
			return fmt.Errorf("parse env file line %d: %w", lineNumber, err)
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read env file: %w", err)
	}

	return nil
}

func validKey(key string) bool {
	for i, r := range key {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}

	return true
}

func unquote(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, `'`) {
		quote := value[0]
		if len(value) < 2 || value[len(value)-1] != quote {
			return "", fmt.Errorf("unterminated quoted value")
		}

		value = value[1 : len(value)-1]
		if quote == '"' {
			value = strings.ReplaceAll(value, `\n`, "\n")
			value = strings.ReplaceAll(value, `\"`, `"`)
			value = strings.ReplaceAll(value, `\\`, `\`)
		}
		return value, nil
	}

	if index := strings.Index(value, " #"); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}

	return value, nil
}
