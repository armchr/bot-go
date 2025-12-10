package change

import (
	"context"

	"bot-go/internal/signals"
	"bot-go/internal/signals/util"
)

// CMSignal computes Changing Methods (method co-change frequency)
type CMSignal struct {
	gitAnalyzer util.GitAnalyzer
}

// NewCMSignal creates a new CM signal
func NewCMSignal(gitAnalyzer util.GitAnalyzer) *CMSignal {
	return &CMSignal{
		gitAnalyzer: gitAnalyzer,
	}
}

// Metadata returns information about this signal
func (s *CMSignal) Metadata() signals.SignalMetadata {
	return signals.SignalMetadata{
		Name:        "CM",
		FullName:    "Changing Methods",
		Category:    signals.CategoryChange,
		Scope:       signals.ScopeMethod,
		Description: "Number of methods that change together with this method across commits",
		Unit:        "count",
		LowerBetter: true, // Fewer co-changing methods means less coupling
	}
}

// Dependencies returns names of signals this signal depends on
func (s *CMSignal) Dependencies() []string {
	return nil
}

// ComputeMethod computes CM for a method
func (s *CMSignal) ComputeMethod(ctx context.Context, methodInfo *signals.MethodInfo, sctx *signals.SignalContext) (signals.SignalResult, error) {
	return signals.SignalResult{}, nil
}
