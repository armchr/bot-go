package cohesion

import (
	"context"

	"bot-go/internal/signals"
	"bot-go/internal/signals/utils"
)

// TCCSignal measures Tight Class Cohesion
// TCC = Number of Directly Connected method pairs / Total possible pairs
// Two methods are directly connected if they access at least one common field
type TCCSignal struct {
	fieldAnalyzer *utils.FieldAccessAnalyzer
}

// NewTCCSignal creates a new TCC signal
func NewTCCSignal() *TCCSignal {
	return &TCCSignal{
		fieldAnalyzer: utils.NewFieldAccessAnalyzer(),
	}
}

func (s *TCCSignal) Name() string {
	return "TCC"
}

func (s *TCCSignal) Category() signals.SignalCategory {
	return signals.CategoryCohesion
}

func (s *TCCSignal) Description() string {
	return "Tight Class Cohesion - ratio of directly connected method pairs to total pairs"
}

func (s *TCCSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	// Ensure field access is analyzed
	s.fieldAnalyzer.AnalyzeFieldAccess(classInfo)

	// Get non-accessor methods (TCC only considers non-trivial methods)
	methods := classInfo.GetNonAccessorMethods()
	n := len(methods)

	// If 0 or 1 method, cohesion is perfect (1.0) or undefined
	if n <= 1 {
		return 1.0, nil
	}

	// Calculate total possible pairs: n(n-1)/2
	totalPairs := n * (n - 1) / 2

	// Count directly connected pairs
	connectedPairs := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if s.fieldAnalyzer.DoMethodsShareFields(methods[i], methods[j]) {
				connectedPairs++
			}
		}
	}

	// Calculate TCC
	if totalPairs == 0 {
		return 1.0, nil
	}

	tcc := float64(connectedPairs) / float64(totalPairs)
	return tcc, nil
}
