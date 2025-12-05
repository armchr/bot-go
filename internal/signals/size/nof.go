package size

import (
	"context"

	"bot-go/internal/signals"
)

// NOFSignal measures Number of Fields
type NOFSignal struct{}

// NewNOFSignal creates a new NOF signal
func NewNOFSignal() *NOFSignal {
	return &NOFSignal{}
}

func (s *NOFSignal) Name() string {
	return "NOF"
}

func (s *NOFSignal) Category() signals.SignalCategory {
	return signals.CategorySize
}

func (s *NOFSignal) Description() string {
	return "Number of Fields - total count of instance variables/fields in the class"
}

func (s *NOFSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	return float64(classInfo.GetNOF()), nil
}
