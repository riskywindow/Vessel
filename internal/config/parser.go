package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Load reads and parses a vessel.toml configuration file.
// It applies defaults and validates the configuration.
func Load(path string) (*Config, error) {
	cfg, err := Parse(path)
	if err != nil {
		return nil, err
	}

	ApplyDefaults(cfg)

	if errs := Validate(cfg); len(errs) > 0 {
		return nil, &ValidationErrors{Errors: errs}
	}

	return cfg, nil
}

// Parse reads and parses a TOML configuration file without applying defaults or validation.
func Parse(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	return ParseBytes(data)
}

// ParseBytes parses TOML configuration from bytes.
func ParseBytes(data []byte) (*Config, error) {
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	return &cfg, nil
}

// ValidationErrors holds multiple validation errors.
type ValidationErrors struct {
	Errors []error
}

// Error implements the error interface.
func (ve *ValidationErrors) Error() string {
	if len(ve.Errors) == 1 {
		return ve.Errors[0].Error()
	}
	return fmt.Sprintf("configuration has %d validation errors", len(ve.Errors))
}

// Unwrap returns the list of errors for use with errors.Is/As.
func (ve *ValidationErrors) Unwrap() []error {
	return ve.Errors
}
