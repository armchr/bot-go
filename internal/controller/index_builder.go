package controller

import (
	"bot-go/internal/config"
	"bot-go/internal/util"
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// IndexBuilder orchestrates the building of various indexes (code graph, embeddings, n-gram)
// for a repository using a parallel file processing approach
type IndexBuilder struct {
	config     *config.Config
	processors []FileProcessor
	logger     *zap.Logger
}

// NewIndexBuilder creates a new index builder with the specified processors
func NewIndexBuilder(config *config.Config, processors []FileProcessor, logger *zap.Logger) *IndexBuilder {
	return &IndexBuilder{
		config:     config,
		processors: processors,
		logger:     logger,
	}
}

// BuildIndex processes a repository through all registered processors
func (ib *IndexBuilder) BuildIndex(ctx context.Context, repo *config.Repository) error {
	return ib.BuildIndexWithGitInfo(ctx, repo, false, nil)
}

// BuildIndexWithGitInfo processes a repository with optional git HEAD optimization
func (ib *IndexBuilder) BuildIndexWithGitInfo(ctx context.Context, repo *config.Repository, useHead bool, gitInfo *util.GitInfo) error {
	if len(ib.processors) == 0 {
		ib.logger.Warn("No processors registered, skipping index building",
			zap.String("repo_name", repo.Name))
		return nil
	}

	ib.logger.Info("Starting index building for repository",
		zap.String("repo_name", repo.Name),
		zap.String("path", repo.Path),
		zap.Int("processor_count", len(ib.processors)))

	// Log which processors are active
	processorNames := make([]string, len(ib.processors))
	for i, p := range ib.processors {
		processorNames[i] = p.Name()
	}
	ib.logger.Info("Active processors", zap.Strings("processors", processorNames))

	// Log git info if using HEAD
	if useHead && gitInfo != nil && gitInfo.IsGitRepo {
		ib.logger.Info("Using git HEAD for index building",
			zap.String("commit_sha", gitInfo.HeadCommitSHA),
			zap.String("commit_msg", gitInfo.HeadCommitMsg),
			zap.Int("modified_files", len(gitInfo.ModifiedFiles)))
	}

	// Phase 1: Process all files in parallel
	err := ib.processFiles(ctx, repo, useHead, gitInfo)
	if err != nil {
		return fmt.Errorf("failed to process files for repository %s: %w", repo.Name, err)
	}

	// Phase 2: Run post-processing steps in parallel
	err = ib.postProcessRepository(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to post-process repository %s: %w", repo.Name, err)
	}

	ib.logger.Info("Completed index building for repository",
		zap.String("repo_name", repo.Name))
	return nil
}

// processFiles walks the repository directory and processes each file through all processors in parallel
func (ib *IndexBuilder) processFiles(ctx context.Context, repo *config.Repository, useHead bool, gitInfo *util.GitInfo) error {
	ib.logger.Info("Processing files",
		zap.String("repo_name", repo.Name),
		zap.String("path", repo.Path))

	fileCount := 0
	filesFromGit := 0
	filesFromDisk := 0
	var mu sync.Mutex

	// Get configuration for WalkDirTree
	gcThreshold := ib.config.App.GCThreshold
	if gcThreshold == 0 {
		gcThreshold = 100 // default
	}

	numThreads := ib.config.App.NumFileThreads
	if numThreads == 0 {
		numThreads = 2 // default
	}

	// Define the skip function for WalkDirTree
	skipFunc := func(path string, isDir bool) bool {
		// Skip hidden directories and common directories to ignore
		if isDir {
			return util.ShouldSkipDirectory(path)
		}
		// Don't skip files here - let individual processors decide
		return false
	}

	// Define the walk function that processes each file
	walkFunc := func(filePath string, err error) error {
		if err != nil {
			ib.logger.Error("Error accessing file", zap.String("path", filePath), zap.Error(err))
			return nil // Continue processing other files
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read file content once, centrally
		// Use optimized reading if useHead is enabled (read from git HEAD for unmodified files)
		content, err := util.ReadFileOptimized(repo.Path, filePath, useHead, gitInfo)
		if err != nil {
			ib.logger.Error("Failed to read file", zap.String("path", filePath), zap.Error(err))
			return nil // Continue processing other files
		}

		// Track source of file content for logging
		if useHead && gitInfo != nil && gitInfo.IsGitRepo {
			mu.Lock()
			if util.IsFileModified(gitInfo, filePath) {
				filesFromDisk++
			} else {
				filesFromGit++
			}
			mu.Unlock()
		}

		// Process the file through all processors in parallel
		var wg sync.WaitGroup
		for _, processor := range ib.processors {
			wg.Add(1)
			go func(p FileProcessor) {
				defer wg.Done()
				if err := p.ProcessFile(ctx, repo, filePath, content); err != nil {
					ib.logger.Error("Processor failed to process file",
						zap.String("processor", p.Name()),
						zap.String("path", filePath),
						zap.Error(err))
				}
			}(processor)
		}
		wg.Wait()

		// Increment file count
		mu.Lock()
		fileCount++
		mu.Unlock()

		return nil
	}

	// Walk the directory tree using the utility function
	err := util.WalkDirTree(repo.Path, walkFunc, skipFunc, ib.logger, gcThreshold, numThreads)
	if err != nil {
		return fmt.Errorf("failed to walk directory tree: %w", err)
	}

	if useHead && gitInfo != nil && gitInfo.IsGitRepo {
		ib.logger.Info("Completed file processing",
			zap.String("repo_name", repo.Name),
			zap.Int("files_processed", fileCount),
			zap.Int("files_from_git_head", filesFromGit),
			zap.Int("files_from_disk", filesFromDisk))
	} else {
		ib.logger.Info("Completed file processing",
			zap.String("repo_name", repo.Name),
			zap.Int("files_processed", fileCount))
	}

	return nil
}

// postProcessRepository runs post-processing steps for all processors in parallel
func (ib *IndexBuilder) postProcessRepository(ctx context.Context, repo *config.Repository) error {
	ib.logger.Info("Running post-processing steps",
		zap.String("repo_name", repo.Name))

	var wg sync.WaitGroup
	errChan := make(chan error, len(ib.processors))

	// Run each processor's post-processing in parallel
	for _, processor := range ib.processors {
		wg.Add(1)
		go func(p FileProcessor) {
			defer wg.Done()
			ib.logger.Info("Starting post-processing",
				zap.String("processor", p.Name()),
				zap.String("repo_name", repo.Name))

			if err := p.PostProcess(ctx, repo); err != nil {
				ib.logger.Error("Post-processing failed",
					zap.String("processor", p.Name()),
					zap.String("repo_name", repo.Name),
					zap.Error(err))
				errChan <- fmt.Errorf("processor %s post-processing failed: %w", p.Name(), err)
				return
			}

			ib.logger.Info("Completed post-processing",
				zap.String("processor", p.Name()),
				zap.String("repo_name", repo.Name))
		}(processor)
	}

	// Wait for all post-processing to complete
	wg.Wait()
	close(errChan)

	// Check if any errors occurred
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("post-processing encountered %d error(s): %v", len(errors), errors)
	}

	ib.logger.Info("Completed all post-processing steps",
		zap.String("repo_name", repo.Name))

	return nil
}
