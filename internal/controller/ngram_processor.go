package controller

import (
	"bot-go/internal/config"
	"bot-go/internal/service/ngram"
	"context"
	"sync/atomic"

	"go.uber.org/zap"
)

// NGramProcessor implements FileProcessor for n-gram model building
type NGramProcessor struct {
	ngramService *ngram.NGramService
	logger       *zap.Logger
	n            int  // N-gram size (e.g., 3 for trigrams)
	override     bool // Whether to override existing models
	fileCount    atomic.Int64
}

// NewNGramProcessor creates a new n-gram processor
func NewNGramProcessor(ngramService *ngram.NGramService, n int, override bool, logger *zap.Logger) *NGramProcessor {
	return &NGramProcessor{
		ngramService: ngramService,
		logger:       logger,
		n:            n,
		override:     override,
	}
}

// Name returns the processor name
func (np *NGramProcessor) Name() string {
	return "NGram"
}

// ProcessFile processes a single file for n-gram model building
func (np *NGramProcessor) ProcessFile(ctx context.Context, repo *config.Repository, filePath string, content []byte) error {
	np.logger.Debug("Processing file for n-gram model", zap.String("path", filePath))

	// The actual file processing happens in the service's ProcessRepository method
	// which handles tokenization and n-gram extraction
	// Here we just track that we've seen the file
	// The content parameter is provided for consistency but not used here
	np.fileCount.Add(1)

	np.logger.Debug("Tracked file for n-gram processing", zap.String("path", filePath))
	return nil
}

// PostProcess performs n-gram model building for the entire repository
func (np *NGramProcessor) PostProcess(ctx context.Context, repo *config.Repository) error {
	np.logger.Info("Building n-gram model",
		zap.String("repo_name", repo.Name),
		zap.Int("n", np.n))

	err := np.ngramService.ProcessRepository(ctx, repo, np.n, np.override)
	if err != nil {
		np.logger.Error("Failed to build n-gram model",
			zap.String("repo_name", repo.Name),
			zap.Error(err))
		return err
	}

	// Get and log statistics
	stats, err := np.ngramService.GetRepositoryStats(ctx, repo.Name)
	if err != nil {
		np.logger.Error("Failed to get n-gram stats",
			zap.String("repo_name", repo.Name),
			zap.Error(err))
	} else {
		np.logger.Info("N-gram model built successfully",
			zap.String("repo_name", repo.Name),
			zap.Int("n", np.n),
			zap.Int("files", stats.TotalFiles),
			zap.Int("tokens", stats.TotalTokens))
	}

	// Reset counter for next repository
	np.fileCount.Store(0)
	return nil
}
