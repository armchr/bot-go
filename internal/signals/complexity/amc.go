package complexity

import (
	"context"

	"bot-go/internal/signals"
)

// AMCSignal measures Average Method Complexity
type AMCSignal struct {
	wmcSignal *WMCSignal
}

// NewAMCSignal creates a new AMC signal
func NewAMCSignal() *AMCSignal {
	return &AMCSignal{
		wmcSignal: NewWMCSignal(),
	}
}

func (s *AMCSignal) Name() string {
	return "AMC"
}

func (s *AMCSignal) Category() signals.SignalCategory {
	return signals.CategoryComplexity
}

func (s *AMCSignal) Description() string {
	return "Average Method Complexity - WMC divided by number of methods"
}

func (s *AMCSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	nom := float64(classInfo.GetNOM())
	if nom == 0 {
		return 0, nil
	}

	wmc, err := s.wmcSignal.Calculate(ctx, classInfo)
	if err != nil {
		return 0, err
	}

	return wmc / nom, nil
}
