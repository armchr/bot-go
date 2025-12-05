package complexity

import (
	"context"

	"bot-go/internal/signals"
	"bot-go/internal/signals/utils"
)

// WMCSignal measures Weighted Method Count (sum of cyclomatic complexity)
type WMCSignal struct{}

// NewWMCSignal creates a new WMC signal
func NewWMCSignal() *WMCSignal {
	return &WMCSignal{}
}

func (s *WMCSignal) Name() string {
	return "WMC"
}

func (s *WMCSignal) Category() signals.SignalCategory {
	return signals.CategoryComplexity
}

func (s *WMCSignal) Description() string {
	return "Weighted Method Count - sum of cyclomatic complexity of all methods"
}

func (s *WMCSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	totalComplexity := 0

	// Create calculator with code graph for graph-based complexity calculation
	calculator := utils.NewComplexityCalculator(classInfo.CodeGraph)

	for _, method := range classInfo.Methods {
		// Calculate complexity if not already computed
		if method.Complexity == -1 {
			// Try graph-based calculation first if we have a valid node
			if method.Node != nil && classInfo.CodeGraph != nil {
				complexity, err := calculator.Calculate(ctx, method.Node.ID)
				if err == nil {
					method.Complexity = complexity
				} else {
					// Fall back to source-based calculation
					method.Complexity = calculator.CalculateFromSource(method.SourceCode)
				}
			} else {
				// Fall back to source-based calculation
				method.Complexity = calculator.CalculateFromSource(method.SourceCode)
			}
		}
		totalComplexity += method.Complexity
	}

	return float64(totalComplexity), nil
}

// WMCNAMMSignal measures WMC without Accessors/Mutators
type WMCNAMMSignal struct{}

// NewWMCNAMMSignal creates a new WMCNAMM signal
func NewWMCNAMMSignal() *WMCNAMMSignal {
	return &WMCNAMMSignal{}
}

func (s *WMCNAMMSignal) Name() string {
	return "WMCNAMM"
}

func (s *WMCNAMMSignal) Category() signals.SignalCategory {
	return signals.CategoryComplexity
}

func (s *WMCNAMMSignal) Description() string {
	return "Weighted Method Count without Accessors/Mutators - WMC excluding simple getters/setters"
}

func (s *WMCNAMMSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	totalComplexity := 0

	// Create calculator with code graph for graph-based complexity calculation
	calculator := utils.NewComplexityCalculator(classInfo.CodeGraph)

	// Only count non-accessor methods
	for _, method := range classInfo.GetNonAccessorMethods() {
		// Calculate complexity if not already computed
		if method.Complexity == -1 {
			// Try graph-based calculation first if we have a valid node
			if method.Node != nil && classInfo.CodeGraph != nil {
				complexity, err := calculator.Calculate(ctx, method.Node.ID)
				if err == nil {
					method.Complexity = complexity
				} else {
					// Fall back to source-based calculation
					method.Complexity = calculator.CalculateFromSource(method.SourceCode)
				}
			} else {
				// Fall back to source-based calculation
				method.Complexity = calculator.CalculateFromSource(method.SourceCode)
			}
		}
		totalComplexity += method.Complexity
	}

	return float64(totalComplexity), nil
}
