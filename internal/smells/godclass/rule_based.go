package godclass

import (
	"context"
	"fmt"

	"bot-go/internal/smells"
)

// RuleBasedStrategy implements traditional rule-based god class detection
// Based on Lanza & Marinescu approach
type RuleBasedStrategy struct{}

// NewRuleBasedStrategy creates a new rule-based strategy
func NewRuleBasedStrategy() *RuleBasedStrategy {
	return &RuleBasedStrategy{}
}

func (s *RuleBasedStrategy) Name() string {
	return "rule_based"
}

func (s *RuleBasedStrategy) Detect(ctx context.Context, signalValues map[string]float64) (*StrategyResult, error) {
	result := &StrategyResult{
		IsGodClass:      false,
		Severity:        smells.SeverityLow,
		Confidence:      0.0,
		ViolatedSignals: []string{},
		Explanation:     "",
	}

	// Check 5-condition rule (Extended)
	violationCount := 0
	var violations []string

	// Condition 1: LOCNAMM ≥ 176
	if locnamm, ok := signalValues["LOCNAMM"]; ok && locnamm >= ThresholdLOCNAMM {
		violationCount++
		violations = append(violations, fmt.Sprintf("LOCNAMM (%.0f ≥ %.0f)", locnamm, ThresholdLOCNAMM))
	}

	// Condition 2: WMCNAMM ≥ 22
	if wmcnamm, ok := signalValues["WMCNAMM"]; ok && wmcnamm >= ThresholdWMCNAMM {
		violationCount++
		violations = append(violations, fmt.Sprintf("WMCNAMM (%.0f ≥ %.0f)", wmcnamm, ThresholdWMCNAMM))
	}

	// Condition 3: NOMNAMM ≥ 18
	if nomnamm, ok := signalValues["NOMNAMM"]; ok && nomnamm >= ThresholdNOMAMM {
		violationCount++
		violations = append(violations, fmt.Sprintf("NOMNAMM (%.0f ≥ %.0f)", nomnamm, ThresholdNOMAMM))
	}

	// Condition 4: TCC ≤ 0.33 (low cohesion)
	if tcc, ok := signalValues["TCC"]; ok && tcc <= ThresholdTCCLow {
		violationCount++
		violations = append(violations, fmt.Sprintf("TCC (%.2f ≤ %.2f)", tcc, ThresholdTCCLow))
	}

	// Condition 5: ATFD ≥ 6
	if atfd, ok := signalValues["ATFD"]; ok && atfd >= ThresholdATFD {
		violationCount++
		violations = append(violations, fmt.Sprintf("ATFD (%.0f ≥ %.0f)", atfd, ThresholdATFD))
	}

	result.ViolatedSignals = violations

	// Determine if god class based on violation count
	if violationCount >= 5 {
		result.IsGodClass = true
		result.Severity = smells.SeverityCritical
		result.Confidence = 0.95
		result.Explanation = "All 5 god class conditions met (critical)"
	} else if violationCount >= 4 {
		result.IsGodClass = true
		result.Severity = smells.SeverityHigh
		result.Confidence = 0.80
		result.Explanation = fmt.Sprintf("4 of 5 god class conditions met (high confidence)")
	} else if violationCount >= 3 {
		result.IsGodClass = true
		result.Severity = smells.SeverityMedium
		result.Confidence = 0.65
		result.Explanation = fmt.Sprintf("3 of 5 god class conditions met (moderate confidence)")
	} else {
		result.IsGodClass = false
		result.Severity = smells.SeverityLow
		result.Confidence = 0.0
		result.Explanation = fmt.Sprintf("Only %d of 5 god class conditions met (not a god class)", violationCount)
	}

	return result, nil
}
