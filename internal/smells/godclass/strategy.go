package godclass

import (
	"context"

	"bot-go/internal/smells"
)

// Strategy defines a god class detection approach
type Strategy interface {
	// Name returns the strategy name
	Name() string

	// Detect analyzes signal values and determines if class is a god class
	Detect(ctx context.Context, signalValues map[string]float64) (*StrategyResult, error)
}

// StrategyResult contains strategy-specific outcome
type StrategyResult struct {
	IsGodClass       bool
	Severity         smells.Severity
	Confidence       float64
	ViolatedSignals  []string
	Explanation      string
}
