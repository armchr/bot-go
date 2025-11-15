package config

import (
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
