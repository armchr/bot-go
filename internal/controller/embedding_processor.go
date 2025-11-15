package controller

import (
	"bot-go/internal/config"
	"bot-go/internal/service/vector"
	"context"
	"sync/atomic"

	"go.uber.org/zap"
)

// EmbeddingProcessor implements FileProcessor for code chunk embeddings
type EmbeddingProcessor struct {
	chunkService *vector.CodeChunkService
	logger       *zap.Logger
	chunkCount   atomic.Int64
}

// NewEmbeddingProcessor creates a new embedding processor
func NewEmbeddingProcessor(chunkService *vector.CodeChunkService, logger *zap.Logger) *EmbeddingProcessor {
	return &EmbeddingProcessor{
		chunkService: chunkService,
		logger:       logger,
	}
}

// Name returns the processor name
func (ep *EmbeddingProcessor) Name() string {
	return "Embedding"
}

// ProcessFile processes a single file for embedding generation
func (ep *EmbeddingProcessor) ProcessFile(ctx context.Context, repo *config.Repository, filePath string, content []byte) error {
	ep.logger.Debug("Processing file for embeddings", zap.String("path", filePath))

	collectionName := repo.Name
	chunks, err := ep.chunkService.ProcessFileWithContent(ctx, filePath, repo.Language, collectionName, content)
	if err != nil {
		ep.logger.Error("Failed to process file for embeddings",
			zap.String("path", filePath),
			zap.Error(err))
		return nil // Continue processing other files
	}

	// Track total chunks processed
	ep.chunkCount.Add(int64(len(chunks)))

	ep.logger.Debug("Successfully processed file for embeddings",
		zap.String("path", filePath),
		zap.Int("chunks", len(chunks)))
	return nil
}

// PostProcess performs any cleanup or finalization after all files are processed
func (ep *EmbeddingProcessor) PostProcess(ctx context.Context, repo *config.Repository) error {
	totalChunks := ep.chunkCount.Load()
	ep.logger.Info("Embedding processing completed",
		zap.String("repo_name", repo.Name),
		zap.Int64("total_chunks", totalChunks))

	// Reset counter for next repository
	ep.chunkCount.Store(0)
	return nil
}
