package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnvFile_Standard(t *testing.T) {
	content := `KEY1=value1
KEY2=value2
DATABASE_URL=postgres://localhost:5432/mydb`

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	env, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["KEY1"] != "value1" {
		t.Errorf("expected KEY1=value1, got %q", env["KEY1"])
	}
	if env["KEY2"] != "value2" {
		t.Errorf("expected KEY2=value2, got %q", env["KEY2"])
	}
	if env["DATABASE_URL"] != "postgres://localhost:5432/mydb" {
		t.Errorf("expected DATABASE_URL, got %q", env["DATABASE_URL"])
	}
}

func TestParseEnvFile_CommentsAndBlanks(t *testing.T) {
	content := `# This is a comment
KEY1=value1

# Another comment
KEY2=value2

`
	scanner := bufio.NewScanner(strings.NewReader(content))
	env, err := ParseEnvReader(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(env) != 2 {
		t.Errorf("expected 2 keys, got %d", len(env))
	}
	if env["KEY1"] != "value1" {
		t.Errorf("expected KEY1=value1, got %q", env["KEY1"])
	}
}

func TestParseEnvFile_QuotedValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		key      string
		expected string
	}{
		{
			name:     "double quotes",
			input:    `KEY="value with spaces"`,
			key:      "KEY",
			expected: "value with spaces",
		},
		{
			name:     "single quotes",
			input:    `KEY='value with spaces'`,
			key:      "KEY",
			expected: "value with spaces",
		},
		{
			name:     "empty double quotes",
			input:    `KEY=""`,
			key:      "KEY",
			expected: "",
		},
		{
			name:     "unquoted value",
			input:    `KEY=simple`,
			key:      "KEY",
			expected: "simple",
		},
		{
			name:     "value with equals sign",
			input:    `KEY=a=b=c`,
			key:      "KEY",
			expected: "a=b=c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			env, err := ParseEnvReader(scanner)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if env[tt.key] != tt.expected {
				t.Errorf("expected %q=%q, got %q", tt.key, tt.expected, env[tt.key])
			}
		})
	}
}

func TestParseEnvFile_VariableReferences(t *testing.T) {
	content := `BASE_URL=https://example.com
API_URL=${BASE_URL}/api
NESTED=${API_URL}/v1`

	scanner := bufio.NewScanner(strings.NewReader(content))
	env, err := ParseEnvReader(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["BASE_URL"] != "https://example.com" {
		t.Errorf("expected BASE_URL, got %q", env["BASE_URL"])
	}
	if env["API_URL"] != "https://example.com/api" {
		t.Errorf("expected API_URL=https://example.com/api, got %q", env["API_URL"])
	}
	if env["NESTED"] != "https://example.com/api/v1" {
		t.Errorf("expected NESTED=https://example.com/api/v1, got %q", env["NESTED"])
	}
}

func TestParseEnvFile_SecretRefsNotResolved(t *testing.T) {
	// Secret references (${secret:key}) should not be resolved at parse time
	content := `API_KEY=${secret:my-api-key}
PLAIN=hello`

	scanner := bufio.NewScanner(strings.NewReader(content))
	env, err := ParseEnvReader(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["API_KEY"] != "${secret:my-api-key}" {
		t.Errorf("expected secret ref to be preserved, got %q", env["API_KEY"])
	}
	if env["PLAIN"] != "hello" {
		t.Errorf("expected PLAIN=hello, got %q", env["PLAIN"])
	}
}

func TestParseEnvFile_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no equals", "INVALID"},
		{"empty key", "=value"},
		{"key starts with number", "1KEY=value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			_, err := ParseEnvReader(scanner)
			if err == nil {
				t.Error("expected error for invalid format")
			}
		})
	}
}

func TestParseEnvFile_Missing(t *testing.T) {
	_, err := ParseEnvFile("/nonexistent/.env")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestMergeEnv(t *testing.T) {
	base := map[string]string{"A": "1", "B": "2"}
	override := map[string]string{"B": "3", "C": "4"}
	highest := map[string]string{"C": "5"}

	result := MergeEnv(base, override, highest)

	if result["A"] != "1" {
		t.Errorf("expected A=1, got %q", result["A"])
	}
	if result["B"] != "3" {
		t.Errorf("expected B=3 (overridden), got %q", result["B"])
	}
	if result["C"] != "5" {
		t.Errorf("expected C=5 (highest priority), got %q", result["C"])
	}
}

func TestMergeEnv_NilMaps(t *testing.T) {
	result := MergeEnv(nil, map[string]string{"A": "1"}, nil)
	if result["A"] != "1" {
		t.Errorf("expected A=1, got %q", result["A"])
	}
}

func TestParseCLIEnvFlags(t *testing.T) {
	flags := []string{"FOO=bar", "BAZ=qux", "EMPTY="}
	env, err := ParseCLIEnvFlags(flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got %q", env["FOO"])
	}
	if env["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got %q", env["BAZ"])
	}
	if env["EMPTY"] != "" {
		t.Errorf("expected EMPTY='', got %q", env["EMPTY"])
	}
}

func TestParseCLIEnvFlags_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
	}{
		{"no equals", []string{"INVALID"}},
		{"empty key", []string{"=value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCLIEnvFlags(tt.flags)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}
