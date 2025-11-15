package controller

import (
	"bot-go/internal/config"
	"bot-go/internal/parse"
	"bot-go/internal/service"
	"bot-go/internal/service/codegraph"
	"context"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CodeGraphProcessor implements FileProcessor for code graph building
type CodeGraphProcessor struct {
	config      *config.Config
	codeGraph   *codegraph.CodeGraph
	repoService *service.RepoService
	logger      *zap.Logger
	fileIDMutex sync.Mutex
}

// NewCodeGraphProcessor creates a new code graph processor
func NewCodeGraphProcessor(
	config *config.Config,
	codeGraph *codegraph.CodeGraph,
	repoService *service.RepoService,
	logger *zap.Logger,
) *CodeGraphProcessor {
	return &CodeGraphProcessor{
		config:      config,
		codeGraph:   codeGraph,
		repoService: repoService,
		logger:      logger,
	}
}

// Name returns the processor name
func (cgp *CodeGraphProcessor) Name() string {
	return "CodeGraph"
}

// ProcessFile processes a single file for code graph building
func (cgp *CodeGraphProcessor) ProcessFile(ctx context.Context, repo *config.Repository, filePath string, content []byte) error {
	fileParser := parse.NewFileParser(cgp.logger, cgp.codeGraph)

	// Create a minimal FileInfo for compatibility (we don't need stat anymore)
	// We'll use a dummy FileInfo that only provides what's needed
	info := &dummyFileInfo{}

	if fileParser.ShouldSkipFile(ctx, repo, info, filePath) {
		return nil
	}

	cgp.logger.Debug("Parsing file for code graph", zap.String("path", filePath))

	// Generate a unique file ID based on the file path
	fileID := cgp.generateFileID(ctx, filePath)
	version := int32(1) // Default version

	err := fileParser.ParseAndTraverseWithContent(ctx, repo, info, filePath, fileID, version, content)
	if err != nil {
		cgp.logger.Error("Failed to parse file for code graph", zap.String("path", filePath), zap.Error(err))
		return nil // Continue processing other files
	}

	cgp.logger.Debug("Successfully parsed file for code graph", zap.String("path", filePath))
	return nil
}

// PostProcess performs LSP-based post-processing on the repository
func (cgp *CodeGraphProcessor) PostProcess(ctx context.Context, repo *config.Repository) error {
	cgp.logger.Info("Running code graph post-processing", zap.String("repo_name", repo.Name))

	postProcessor := NewPostProcessor(cgp.codeGraph, cgp.repoService.GetLspService(), cgp.logger)
	err := postProcessor.PostProcessRepository(ctx, repo)
	if err != nil {
		cgp.logger.Error("Code graph post-processing failed",
			zap.String("repo_name", repo.Name),
			zap.Error(err))
		return err
	}

	cgp.logger.Info("Code graph post-processing completed", zap.String("repo_name", repo.Name))
	return nil
}

// generateFileID generates a unique file ID for the given file path
func (cgp *CodeGraphProcessor) generateFileID(ctx context.Context, filePath string) int32 {
	cgp.fileIDMutex.Lock()
	defer cgp.fileIDMutex.Unlock()

	fileID, err := cgp.codeGraph.GetOrCreateNextFileID(ctx)
	if err != nil {
		cgp.logger.Error("Failed to generate file ID", zap.String("path", filePath), zap.Error(err))
		return 0
	}
	return fileID
}

// dummyFileInfo is a minimal implementation of os.FileInfo
// Used when we already have file content and don't need to stat the file
type dummyFileInfo struct{}

func (d *dummyFileInfo) Name() string       { return "" }
func (d *dummyFileInfo) Size() int64        { return 0 }
func (d *dummyFileInfo) Mode() os.FileMode  { return 0 }
func (d *dummyFileInfo) ModTime() time.Time { return time.Time{} }
func (d *dummyFileInfo) IsDir() bool        { return false }
func (d *dummyFileInfo) Sys() interface{}   { return nil }
