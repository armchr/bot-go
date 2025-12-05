package size

import (
	"context"

	"bot-go/internal/signals"
)

// NOMSignal measures Number of Methods
type NOMSignal struct{}

// NewNOMSignal creates a new NOM signal
func NewNOMSignal() *NOMSignal {
	return &NOMSignal{}
}

func (s *NOMSignal) Name() string {
	return "NOM"
}

func (s *NOMSignal) Category() signals.SignalCategory {
	return signals.CategorySize
}

func (s *NOMSignal) Description() string {
	return "Number of Methods - total count of methods in the class"
}

func (s *NOMSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	return float64(classInfo.GetNOM()), nil
}

// NOMAMMSignal measures Number of Methods without Accessors/Mutators
type NOMAMMSignal struct{}

// NewNOMAMMSignal creates a new NOMAMM signal
func NewNOMAMMSignal() *NOMAMMSignal {
	return &NOMAMMSignal{}
}

func (s *NOMAMMSignal) Name() string {
	return "NOMNAMM"
}

func (s *NOMAMMSignal) Category() signals.SignalCategory {
	return signals.CategorySize
}

func (s *NOMAMMSignal) Description() string {
	return "Number of Methods without Accessors/Mutators - method count excluding simple getters/setters"
}

func (s *NOMAMMSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	nonAccessorMethods := classInfo.GetNonAccessorMethods()
	return float64(len(nonAccessorMethods)), nil
}
