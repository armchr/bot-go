package config

import (
	"os"
	"testing"
)

func TestKuzuConfig_Parsing(t *testing.T) {
	// Test KuzuConfig struct
	kuzu := KuzuConfig{
		Path: "/path/to/kuzu.db",
	}

	if kuzu.Path != "/path/to/kuzu.db" {
		t.Fatalf("Expected path '/path/to/kuzu.db', got '%s'", kuzu.Path)
	}
}

func TestConfig_KuzuField(t *testing.T) {
	// Test that Config struct has Kuzu field
	config := Config{
		Kuzu: KuzuConfig{
			Path: ":memory:",
		},
	}

	if config.Kuzu.Path != ":memory:" {
		t.Fatalf("Expected Kuzu path ':memory:', got '%s'", config.Kuzu.Path)
	}
}

func TestExpandEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "Simple ${VAR} syntax",
			input:    "path: ${HOME}/data",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: "path: /home/user/data",
		},
		{
			name:     "Simple $VAR syntax",
			input:    "path: $HOME/data",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: "path: /home/user/data",
		},
		{
			name:     "${VAR:-default} with env set",
			input:    "path: ${DB_PATH:-/default/path}",
			envVars:  map[string]string{"DB_PATH": "/custom/path"},
			expected: "path: /custom/path",
		},
		{
			name:     "${VAR:-default} with env not set",
			input:    "path: ${DB_PATH:-/default/path}",
			envVars:  map[string]string{},
			expected: "path: /default/path",
		},
		{
			name:     "Multiple variables",
			input:    "uri: ${PROTOCOL}://${HOST}:${PORT}",
			envVars:  map[string]string{"PROTOCOL": "http", "HOST": "localhost", "PORT": "8080"},
			expected: "uri: http://localhost:8080",
		},
		{
			name:     "Mixed syntax",
			input:    "$USER uses ${HOME:-/tmp}",
			envVars:  map[string]string{"USER": "alice", "HOME": "/home/alice"},
			expected: "alice uses /home/alice",
		},
		{
			name:     "Undefined variable without default (${VAR})",
			input:    "path: ${UNDEFINED_VAR}",
			envVars:  map[string]string{},
			expected: "path: ",
		},
		{
			name:     "Undefined variable without default ($VAR)",
			input:    "path: $UNDEFINED_VAR",
			envVars:  map[string]string{},
			expected: "path: $UNDEFINED_VAR",
		},
		{
			name:     "Empty default value",
			input:    "path: ${EMPTY:-}",
			envVars:  map[string]string{},
			expected: "path: ",
		},
		{
			name:     "No variables",
			input:    "path: /static/path",
			envVars:  map[string]string{},
			expected: "path: /static/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Clear any variables we're testing as undefined
			if len(tt.envVars) == 0 && tt.input != "path: /static/path" {
				// Extract variable names from input and unset them
				testVars := []string{"UNDEFINED_VAR", "EMPTY", "DB_PATH"}
				for _, v := range testVars {
					os.Unsetenv(v)
				}
			}

			result := expandEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("expandEnvVars(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
