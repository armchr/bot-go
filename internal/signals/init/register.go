package signalinit

import (
	"bot-go/internal/config"
	"bot-go/internal/signals"
	"bot-go/internal/signals/change"
	"bot-go/internal/signals/cohesion"
	"bot-go/internal/signals/complexity"
	"bot-go/internal/signals/coupling"
	"bot-go/internal/signals/entropy"
	"bot-go/internal/signals/messagechain"
	"bot-go/internal/signals/size"
	"bot-go/internal/signals/util"
	"bot-go/internal/signals/woc"
)

// RegisterDefaultSignals registers all built-in signals with the registry
func RegisterDefaultSignals(registry *signals.SignalRegistry) {
	// Size signals
	registry.Register(size.NewLOCSignal())
	registry.Register(size.NewLOCNAMMSignal())
	registry.Register(size.NewNOMSignal())
	registry.Register(size.NewNOMNAMMSignal())
	registry.Register(size.NewNOFSignal())
	registry.Register(size.NewNOPASignal())
	registry.Register(size.NewNOAMSignal())

	// Complexity signals
	registry.Register(complexity.NewCYCLOSignal())
	registry.Register(complexity.NewWMCSignal())
	registry.Register(complexity.NewWMCNAMMSignal())
	registry.Register(complexity.NewAMCSignal())
	registry.Register(complexity.NewMAXNESTINGSignal())
	registry.Register(complexity.NewNOLVSignal())

	// Cohesion signals
	registry.Register(cohesion.NewTCCSignal())
	registry.Register(cohesion.NewLCCSignal())
	registry.Register(cohesion.NewLCOMSignal())
	registry.Register(cohesion.NewLCOM4Signal())
	registry.Register(cohesion.NewATLDSignal())

	// Coupling signals
	registry.Register(coupling.NewATFDSignal())
	registry.Register(coupling.NewFANOUTSignal())
	registry.Register(coupling.NewCINTSignal())
	registry.Register(coupling.NewCDISPSignal())
	registry.Register(coupling.NewFDPSignal())
	registry.Register(coupling.NewLAASignal())
	registry.Register(coupling.NewCBOSignal())
	registry.Register(coupling.NewRFCSignal())

	// Message chain signals
	registry.Register(messagechain.NewMaMCLSignal())
	registry.Register(messagechain.NewMeMCLSignal())
	registry.Register(messagechain.NewNMCSSignal())

	// Entropy signals
	registry.Register(entropy.NewFileEntropySignal())
	registry.Register(entropy.NewMethodEntropySignal())
	registry.Register(entropy.NewClassEntropySignal())
	registry.Register(entropy.NewZScoreSignal())
	registry.Register(entropy.NewHighEntropyMethodCountSignal(6.0)) // Default threshold

	// Composite signals
	registry.Register(woc.NewWOCSignal())
}

// RegisterChangeSignals registers change history signals (requires git analyzer)
func RegisterChangeSignals(registry *signals.SignalRegistry, gitAnalyzer util.GitAnalyzer) {
	registry.Register(change.NewCCSignal(gitAnalyzer))
	registry.Register(change.NewCMSignal(gitAnalyzer))
}

// RegisterAllSignals registers all signals including change history
// gitConfig can be nil to use default on-demand mode
func RegisterAllSignals(registry *signals.SignalRegistry, repoPath string, gitConfig *config.GitAnalysisConfig) error {
	RegisterDefaultSignals(registry)
	gitAnalyzer, err := util.NewGitAnalyzer(repoPath, gitConfig)
	if err != nil {
		return err
	}
	RegisterChangeSignals(registry, gitAnalyzer)
	return nil
}
