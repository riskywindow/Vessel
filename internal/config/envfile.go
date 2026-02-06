package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseEnvFile reads a .env file and returns the key-value pairs.
// Supports:
//   - KEY=VALUE (standard format)
//   - Comments (# at start of line)
//   - Blank lines
//   - Quoted values: KEY="value with spaces"
//   - Single-quoted values: KEY='value'
//   - Variable references: KEY=${OTHER_KEY} (resolved from same file or existing env)
func ParseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open env file: %w", err)
	}
	defer f.Close()

	return ParseEnvReader(bufio.NewScanner(f))
}

// ParseEnvReader parses env vars from a scanner line by line.
func ParseEnvReader(scanner *bufio.Scanner) (map[string]string, error) {
	env := make(map[string]string)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, err := parseEnvLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		// Resolve variable references from previously parsed values or OS env
		value = resolveVarRefs(value, env)

		env[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	return env, nil
}

// parseEnvLine parses a single KEY=VALUE line.
func parseEnvLine(line string) (string, string, error) {
	// Strip inline comments (only if not inside quotes)
	// Find the = separator first
	eqIdx := strings.IndexByte(line, '=')
	if eqIdx < 0 {
		return "", "", fmt.Errorf("invalid format: expected KEY=VALUE, got %q", line)
	}

	key := strings.TrimSpace(line[:eqIdx])
	if key == "" {
		return "", "", fmt.Errorf("empty key")
	}

	// Validate key: must be [A-Za-z_][A-Za-z0-9_-]*
	for i, c := range key {
		if i == 0 {
			if !isAlpha(c) && c != '_' {
				return "", "", fmt.Errorf("invalid key %q: must start with a letter or underscore", key)
			}
		} else {
			if !isAlphaNum(c) && c != '_' && c != '-' {
				return "", "", fmt.Errorf("invalid key %q: contains invalid character %q", key, c)
			}
		}
	}

	value := strings.TrimSpace(line[eqIdx+1:])

	// Handle quoted values
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	return key, value, nil
}

// resolveVarRefs replaces ${VAR} and $VAR references in a value.
// Looks up from the provided env map first, then falls back to os.Getenv.
func resolveVarRefs(value string, env map[string]string) string {
	// Replace ${VAR} references
	result := value
	for {
		start := strings.Index(result, "${")
		if start < 0 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end < 0 {
			break
		}
		end += start

		varName := result[start+2 : end]
		// Skip secret references — those are handled during deploy
		if strings.HasPrefix(varName, "secret:") {
			// Move past this reference to avoid infinite loop
			break
		}

		varValue := lookupVar(varName, env)
		result = result[:start] + varValue + result[end+1:]
	}

	return result
}

// lookupVar looks up a variable from the env map, then falls back to os.Getenv.
func lookupVar(name string, env map[string]string) string {
	if v, ok := env[name]; ok {
		return v
	}
	return os.Getenv(name)
}

func isAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isAlphaNum(c rune) bool {
	return isAlpha(c) || (c >= '0' && c <= '9')
}

// MergeEnv merges multiple env maps in order. Later maps override earlier ones.
func MergeEnv(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// ParseCLIEnvFlags parses --env KEY=VALUE flags into a map.
func ParseCLIEnvFlags(flags []string) (map[string]string, error) {
	env := make(map[string]string)
	for _, flag := range flags {
		eqIdx := strings.IndexByte(flag, '=')
		if eqIdx < 0 {
			return nil, fmt.Errorf("invalid env flag %q: expected KEY=VALUE format", flag)
		}
		key := flag[:eqIdx]
		value := flag[eqIdx+1:]
		if key == "" {
			return nil, fmt.Errorf("invalid env flag %q: empty key", flag)
		}
		env[key] = value
	}
	return env, nil
}
