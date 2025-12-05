package godclass

import (
	"context"
	"fmt"
	"sync"

	"bot-go/internal/signals"
	"bot-go/internal/signals/cohesion"
	"bot-go/internal/signals/complexity"
	"bot-go/internal/signals/coupling"
	"bot-go/internal/signals/size"
	"bot-go/internal/smells"

	"go.uber.org/zap"
)

// GodClassDetector detects god class code smell
type GodClassDetector struct {
	signalRegistry *signals.SignalRegistry
	strategies     []Strategy
	recommender    *Recommender
	logger         *zap.Logger
}

// NewGodClassDetector creates a new god class detector
func NewGodClassDetector(logger *zap.Logger) *GodClassDetector {
	// Create signal registry and register all needed signals
	signalRegistry := signals.NewSignalRegistry()

	// Register size signals
	signalRegistry.Register(size.NewLOCSignal())
	signalRegistry.Register(size.NewLOCNAMMSignal())
	signalRegistry.Register(size.NewNOMSignal())
	signalRegistry.Register(size.NewNOMAMMSignal())
	signalRegistry.Register(size.NewNOFSignal())

	// Register complexity signals
	signalRegistry.Register(complexity.NewWMCSignal())
	signalRegistry.Register(complexity.NewWMCNAMMSignal())
	signalRegistry.Register(complexity.NewAMCSignal())

	// Register cohesion signals
	signalRegistry.Register(cohesion.NewTCCSignal())

	// Register coupling signals
	signalRegistry.Register(coupling.NewATFDSignal())

	// TODO: Register semantic and statistical signals when implemented

	// Create strategies
	strategies := []Strategy{
		NewRuleBasedStrategy(),
		NewScoreBasedStrategy(),
	}

	// Create recommender
	recommender := NewRecommender(logger)

	return &GodClassDetector{
		signalRegistry: signalRegistry,
		strategies:     strategies,
		recommender:    recommender,
		logger:         logger,
	}
}

func (d *GodClassDetector) Name() string {
	return "god_class_detector"
}

func (d *GodClassDetector) SmellType() smells.SmellType {
	return smells.SmellTypeGodClass
}

// Detect runs god class detection on a class
func (d *GodClassDetector) Detect(ctx context.Context, classInfo *signals.ClassInfo) (*smells.DetectionResult, error) {
	d.logger.Info("Running god class detection",
		zap.String("class", classInfo.ClassName),
		zap.String("repo", classInfo.RepoName))

	// Step 1: Calculate all signals in parallel
	signalValues, err := d.calculateSignals(ctx, classInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate signals: %w", err)
	}

	d.logger.Debug("Calculated signals",
		zap.String("class", classInfo.ClassName),
		zap.Int("signal_count", len(signalValues)))

	// Step 2: Run all strategies
	strategyResults, err := d.runStrategies(ctx, signalValues)
	if err != nil {
		return nil, fmt.Errorf("failed to run strategies: %w", err)
	}

	// Step 3: Aggregate strategy results
	aggregatedResult := d.aggregateResults(strategyResults)

	// Step 4: Create detection result
	result := smells.NewDetectionResult(
		smells.SmellTypeGodClass,
		classInfo.RepoName,
		classInfo.ClassName,
		classInfo.FilePath,
	)

	result.IsSmell = aggregatedResult.IsGodClass
	result.Severity = aggregatedResult.Severity
	result.Confidence = aggregatedResult.Confidence
	result.Strategy = aggregatedResult.BestStrategy
	result.SignalValues = signalValues
	result.ViolatedSignals = aggregatedResult.ViolatedSignals

	// Step 5: Generate recommendations if god class detected
	if result.IsSmell {
		result.Recommendations = d.recommender.Generate(ctx, classInfo, signalValues)
	}

	d.logger.Info("God class detection complete",
		zap.String("class", classInfo.ClassName),
		zap.Bool("is_god_class", result.IsSmell),
		zap.String("severity", string(result.Severity)),
		zap.Float64("confidence", result.Confidence))

	return result, nil
}

// calculateSignals computes all registered signals
func (d *GodClassDetector) calculateSignals(ctx context.Context, classInfo *signals.ClassInfo) (map[string]float64, error) {
	allSignals := d.signalRegistry.GetAll()
	results := make(map[string]float64)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(allSignals))

	// Calculate signals in parallel
	for _, signal := range allSignals {
		wg.Add(1)
		go func(sig signals.Signal) {
			defer wg.Done()

			value, err := sig.Calculate(ctx, classInfo)
			if err != nil {
				d.logger.Warn("Signal calculation failed",
					zap.String("signal", sig.Name()),
					zap.Error(err))
				errChan <- err
				return
			}

			mu.Lock()
			results[sig.Name()] = value
			mu.Unlock()
		}(signal)
	}

	wg.Wait()
	close(errChan)

	// Check if any errors occurred
	if len(errChan) > 0 {
		d.logger.Warn("Some signals failed to calculate, continuing with available signals")
	}

	return results, nil
}

// runStrategies executes all detection strategies
func (d *GodClassDetector) runStrategies(ctx context.Context, signalValues map[string]float64) ([]*StrategyResult, error) {
	results := make([]*StrategyResult, 0, len(d.strategies))

	for _, strategy := range d.strategies {
		result, err := strategy.Detect(ctx, signalValues)
		if err != nil {
			d.logger.Warn("Strategy failed",
				zap.String("strategy", strategy.Name()),
				zap.Error(err))
			continue
		}
		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("all strategies failed")
	}

	return results, nil
}

// AggregatedResult combines results from multiple strategies
type AggregatedResult struct {
	IsGodClass       bool
	Severity         smells.Severity
	Confidence       float64
	BestStrategy     string
	ViolatedSignals  []string
}

// aggregateResults combines results from multiple strategies
func (d *GodClassDetector) aggregateResults(strategyResults []*StrategyResult) *AggregatedResult {
	// Use the highest severity result
	aggregated := &AggregatedResult{
		IsGodClass:      false,
		Severity:        smells.SeverityLow,
		Confidence:      0.0,
		BestStrategy:    "",
		ViolatedSignals: []string{},
	}

	severityOrder := map[smells.Severity]int{
		smells.SeverityCritical: 4,
		smells.SeverityHigh:     3,
		smells.SeverityMedium:   2,
		smells.SeverityLow:      1,
	}

	for _, result := range strategyResults {
		if result.IsGodClass {
			aggregated.IsGodClass = true

			// Pick highest severity
			if severityOrder[result.Severity] > severityOrder[aggregated.Severity] {
				aggregated.Severity = result.Severity
				aggregated.Confidence = result.Confidence
				aggregated.BestStrategy = "rule_based" // Default, could get from result
				aggregated.ViolatedSignals = result.ViolatedSignals
			}
		}
	}

	// If no strategy detected god class, use the best non-detection result
	if !aggregated.IsGodClass && len(strategyResults) > 0 {
		aggregated.BestStrategy = "none"
		aggregated.Confidence = 0.0
	}

	return aggregated
}
