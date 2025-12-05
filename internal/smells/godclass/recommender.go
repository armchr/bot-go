package godclass

import (
	"context"
	"fmt"

	"bot-go/internal/signals"
	"bot-go/internal/smells"

	"go.uber.org/zap"
)

// Recommender generates refactoring recommendations for god classes
type Recommender struct {
	logger *zap.Logger
}

// NewRecommender creates a new recommender
func NewRecommender(logger *zap.Logger) *Recommender {
	return &Recommender{
		logger: logger,
	}
}

// Generate creates refactoring recommendations based on signal values
func (r *Recommender) Generate(ctx context.Context, classInfo *signals.ClassInfo, signalValues map[string]float64) []smells.Recommendation {
	var recommendations []smells.Recommendation

	// Recommendation 1: Extract class (if many methods)
	if nomnamm, ok := signalValues["NOMNAMM"]; ok && nomnamm >= ThresholdNOMAMM {
		recommendations = append(recommendations, smells.Recommendation{
			Type:        "extract_class",
			Description: fmt.Sprintf("Class has %.0f methods (excluding accessors). Consider extracting related methods into separate classes based on their responsibilities.", nomnamm),
			Priority:    1,
			TargetCode:  []string{"Consider grouping methods by functionality"},
		})
	}

	// Recommendation 2: Improve cohesion (if low TCC)
	if tcc, ok := signalValues["TCC"]; ok && tcc <= ThresholdTCCLow {
		recommendations = append(recommendations, smells.Recommendation{
			Type:        "improve_cohesion",
			Description: fmt.Sprintf("Tight Class Cohesion is %.2f (threshold: %.2f). Methods don't work together well. Consider splitting the class based on which methods access which fields.", tcc, ThresholdTCCLow),
			Priority:    1,
			TargetCode:  []string{"Analyze method-field access patterns"},
		})
	}

	// Recommendation 3: Reduce coupling (if high ATFD)
	if atfd, ok := signalValues["ATFD"]; ok && atfd >= ThresholdATFD {
		recommendations = append(recommendations, smells.Recommendation{
			Type:        "reduce_coupling",
			Description: fmt.Sprintf("Class accesses %.0f external attributes (threshold: %.0f). This creates high coupling. Consider using dependency injection or extracting collaborating classes.", atfd, ThresholdATFD),
			Priority:    2,
			TargetCode:  []string{"Review external field accesses"},
		})
	}

	// Recommendation 4: Reduce complexity (if high WMC)
	if wmcnamm, ok := signalValues["WMCNAMM"]; ok && wmcnamm >= ThresholdWMCNAMM {
		recommendations = append(recommendations, smells.Recommendation{
			Type:        "reduce_complexity",
			Description: fmt.Sprintf("Weighted Method Count is %.0f (threshold: %.0f). Methods are too complex. Break down complex methods into smaller, focused methods.", wmcnamm, ThresholdWMCNAMM),
			Priority:    2,
			TargetCode:  r.findComplexMethods(classInfo),
		})
	}

	// Recommendation 5: Reduce size (if large LOC)
	if locnamm, ok := signalValues["LOCNAMM"]; ok && locnamm >= ThresholdLOCNAMM {
		recommendations = append(recommendations, smells.Recommendation{
			Type:        "reduce_size",
			Description: fmt.Sprintf("Class has %.0f lines of code (threshold: %.0f). This is too large. Apply Single Responsibility Principle - each class should have one reason to change.", locnamm, ThresholdLOCNAMM),
			Priority:    1,
			TargetCode:  []string{fmt.Sprintf("Class: %s", classInfo.ClassName)},
		})
	}

	// If no specific recommendations, provide general advice
	if len(recommendations) == 0 {
		recommendations = append(recommendations, smells.Recommendation{
			Type:        "general",
			Description: "Apply the Single Responsibility Principle. Consider splitting this class based on its different responsibilities.",
			Priority:    3,
			TargetCode:  []string{},
		})
	}

	return recommendations
}

// findComplexMethods identifies methods with high cyclomatic complexity
func (r *Recommender) findComplexMethods(classInfo *signals.ClassInfo) []string {
	var complexMethods []string

	for _, method := range classInfo.Methods {
		if method.Complexity > 10 { // Complexity > 10 is considered high
			complexMethods = append(complexMethods, fmt.Sprintf("%s (complexity: %d)", method.Name, method.Complexity))
		}
	}

	if len(complexMethods) == 0 {
		return []string{"Review all methods for complexity"}
	}

	return complexMethods
}
