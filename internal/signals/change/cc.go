package change

import (
	"context"

	"bot-go/internal/signals"
	"bot-go/internal/signals/util"
)

// CCSignal computes Changing Classes (co-change frequency)
type CCSignal struct {
	gitAnalyzer util.GitAnalyzer
}

// NewCCSignal creates a new CC signal
func NewCCSignal(gitAnalyzer util.GitAnalyzer) *CCSignal {
	return &CCSignal{
		gitAnalyzer: gitAnalyzer,
	}
}

// Metadata returns information about this signal
func (s *CCSignal) Metadata() signals.SignalMetadata {
	return signals.SignalMetadata{
		Name:        "CC",
		FullName:    "Changing Classes",
		Category:    signals.CategoryChange,
		Scope:       signals.ScopeClass,
		Description: "Number of classes that change together with this class across commits",
		Unit:        "count",
		LowerBetter: true, // Fewer co-changing classes means less coupling
	}
}

// Dependencies returns names of signals this signal depends on
func (s *CCSignal) Dependencies() []string {
	return nil
}

// ComputeClass computes CC for a class
func (s *CCSignal) ComputeClass(ctx context.Context, classInfo *signals.ClassInfo, sctx *signals.SignalContext) (signals.SignalResult, error) {
	return signals.SignalResult{}, nil
}
