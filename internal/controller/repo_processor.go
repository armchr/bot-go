package controller

import (
	"bot-go/internal/config"
	"bot-go/internal/service"
	"bot-go/internal/service/codegraph"
	"bot-go/internal/util"
	"context"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
)

type RepoProcessor struct {
	config        *config.Config
	codeGraph     *codegraph.CodeGraph
	repoService   *service.RepoService
	logger        *zap.Logger
	codeGraphProc *CodeGraphProcessor
}

func NewRepoProcessor(config *config.Config, codeGraph *codegraph.CodeGraph, logger *zap.Logger) *RepoProcessor {
	return &RepoProcessor{
		config:    config,
		codeGraph: codeGraph,
		logger:    logger,
	}
}

// SetRepoService sets the repository service (needed for post-processing)
func (rp *RepoProcessor) SetRepoService(repoService *service.RepoService) {
	rp.repoService = repoService
	rp.codeGraphProc = NewCodeGraphProcessor(rp.config, rp.codeGraph, repoService, rp.logger)
}

func (rp *RepoProcessor) ProcessRepository(ctx context.Context, repo *config.Repository) error {
	rp.logger.Info("Processing repository", zap.String("name", repo.Name), zap.String("path", repo.Path))

	if rp.codeGraphProc == nil {
		return fmt.Errorf("repo processor not properly initialized - call SetRepoService first")
	}

	// Get configuration for WalkDirTree
	gcThreshold := rp.config.App.GCThreshold
	if gcThreshold == 0 {
		gcThreshold = 100 // default
	}

	numThreads := rp.config.App.NumFileThreads
	if numThreads == 0 {
		numThreads = 2 // default
	}

	fileCount := 0
	var mu sync.Mutex

	// Define the skip function for WalkDirTree
	skipFunc := func(path string, isDir bool) bool {
		// Skip hidden directories and common directories to ignore
		if isDir {
			return util.ShouldSkipDirectory(path)
		}
		// Don't skip files here - let CodeGraphProcessor decide
		return false
	}

	// Define the walk function that processes each file
	walkFunc := func(filePath string, err error) error {
		if err != nil {
			rp.logger.Error("Error accessing file", zap.String("path", filePath), zap.Error(err))
			return nil // Continue processing other files
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read file content once
		content, err := os.ReadFile(filePath)
		if err != nil {
			rp.logger.Error("Failed to read file", zap.String("path", filePath), zap.Error(err))
			return nil // Continue processing other files
		}

		// Process the file through CodeGraphProcessor
		if err := rp.codeGraphProc.ProcessFile(ctx, repo, filePath, content); err != nil {
			rp.logger.Error("CodeGraphProcessor failed to process file",
				zap.String("path", filePath),
				zap.Error(err))
		}

		// Increment file count
		mu.Lock()
		fileCount++
		mu.Unlock()

		return nil
	}

	// Walk the directory tree using the utility function
	err := util.WalkDirTree(repo.Path, walkFunc, skipFunc, rp.logger, gcThreshold, numThreads)
	if err != nil {
		return fmt.Errorf("failed to walk directory tree: %w", err)
	}

	rp.logger.Info("Completed file processing",
		zap.String("repo_name", repo.Name),
		zap.Int("files_processed", fileCount))

	// Run post-processing
	err = rp.codeGraphProc.PostProcess(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to post-process repository %s: %w", repo.Name, err)
	}

	rp.logger.Info("Completed processing repository", zap.String("name", repo.Name))
	return nil
}

func (rp *RepoProcessor) ProcessAllRepositories(ctx context.Context, postProcessor *PostProcessor) error {
	rp.logger.Info("Starting to process all repositories", zap.Int("count", len(rp.config.Source.Repositories)))

	// Note: postProcessor is now integrated into CodeGraphProcessor's PostProcess method
	// The IndexBuilder will automatically call PostProcess after processing all files

	executorPool := util.NewExecutorPool(5, 100, func(task any) {
		repo := task.(*config.Repository)
		err := rp.ProcessRepository(ctx, repo)
		if err != nil {
			rp.logger.Error("Failed to process repository", zap.String("name", repo.Name), zap.Error(err))
			return
		}
		// Post-processing is now handled automatically by IndexBuilder
	})

	defer executorPool.Close()

	for _, repo := range rp.config.Source.Repositories {
		if repo.Disabled {
			rp.logger.Info("Skipping disabled repository", zap.String("name", repo.Name))
			continue
		}
		switch repo.Language {
		case "python":
		//case "typescript", "javascript":
		//case "go", "golang":
		// Supported languages
		default:
			rp.logger.Warn("Skipping unsupported repository language", zap.String("name", repo.Name), zap.String("language", repo.Language))
			continue
		}
		select {
		case <-ctx.Done():
			rp.logger.Info("Context cancelled, stopping repository processing")
			return ctx.Err()
		default:
			executorPool.Submit(&repo)
		}
	}

	rp.logger.Info("Completed processing all repositories")
	return nil
}
