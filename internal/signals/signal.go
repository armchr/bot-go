package signals

import (
	"context"

	"bot-go/internal/signals/model"
)

// Signal represents any measurable metric/signal about code
type Signal interface {
	// Name returns the unique identifier for this signal
	Name() string

	// Category returns the category this signal belongs to
	Category() SignalCategory

	// Calculate computes the signal value for a given class
	Calculate(ctx context.Context, classInfo *model.ClassInfo) (float64, error)

	// Description returns a human-readable description
	Description() string
}

// SignalCategory groups related signals
type SignalCategory string

const (
	CategorySize        SignalCategory = "size"
	CategoryComplexity  SignalCategory = "complexity"
	CategoryCohesion    SignalCategory = "cohesion"
	CategoryCoupling    SignalCategory = "coupling"
	CategorySemantic    SignalCategory = "semantic"
	CategoryStatistical SignalCategory = "statistical"
)

// SignalRegistry manages all available signals
type SignalRegistry struct {
	signals map[string]Signal
}

// NewSignalRegistry creates a new signal registry
func NewSignalRegistry() *SignalRegistry {
	return &SignalRegistry{
		signals: make(map[string]Signal),
	}
}

// Register adds a signal to the registry
func (r *SignalRegistry) Register(signal Signal) {
	r.signals[signal.Name()] = signal
}

// Get retrieves a signal by name
func (r *SignalRegistry) Get(name string) (Signal, bool) {
	signal, ok := r.signals[name]
	return signal, ok
}

// GetByCategory returns all signals in a category
func (r *SignalRegistry) GetByCategory(category SignalCategory) []Signal {
	var result []Signal
	for _, signal := range r.signals {
		if signal.Category() == category {
			result = append(result, signal)
		}
	}
	return result
}

// GetAll returns all registered signals
func (r *SignalRegistry) GetAll() []Signal {
	result := make([]Signal, 0, len(r.signals))
	for _, signal := range r.signals {
		result = append(result, signal)
	}
	return result
}

// CalculateAll calculates all registered signals for a class
func (r *SignalRegistry) CalculateAll(ctx context.Context, classInfo *model.ClassInfo) (map[string]float64, error) {
	results := make(map[string]float64)

	for name, signal := range r.signals {
		value, err := signal.Calculate(ctx, classInfo)
		if err != nil {
			// Log error but continue with other signals
			// Store -1 to indicate calculation failure
			results[name] = -1
			continue
		}
		results[name] = value
	}

	return results, nil
}
