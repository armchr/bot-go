package controller

import (
	"bot-go/internal/config"
	"bot-go/internal/parse"
	"bot-go/internal/service"
	"bot-go/internal/util"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

type RepoProcessor struct {
	config    *config.Config
	codeGraph *service.CodeGraph
	logger    *zap.Logger
}

func NewRepoProcessor(config *config.Config, codeGraph *service.CodeGraph, logger *zap.Logger) *RepoProcessor {
	return &RepoProcessor{
		config:    config,
		codeGraph: codeGraph,
		logger:    logger,
	}
}

func (rp *RepoProcessor) ProcessRepository(ctx context.Context, repo *config.Repository) error {
	rp.logger.Info("Processing repository", zap.String("name", repo.Name), zap.String("path", repo.Path))

	err := filepath.Walk(repo.Path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			rp.logger.Error("Error accessing file", zap.String("path", filePath), zap.Error(err))
			return nil // Continue processing other files
		}

		if info.IsDir() {
			return nil // Skip directories
		}

		fileParser := parse.NewFileParser(rp.logger, rp.codeGraph)

		if fileParser.ShouldSkipFile(ctx, repo, info, filePath) {
			return nil
		}

		rp.logger.Debug("Parsing file", zap.String("path", filePath))

		// Generate a unique file ID based on the file path
		fileID := rp.generateFileID(ctx, filePath)
		version := int32(1) // Default version

		err = fileParser.ParseAndTraverse(ctx, repo, info, filePath, fileID, version)
		if err != nil {
			rp.logger.Error("Failed to parse file", zap.String("path", filePath), zap.Error(err))
			return nil // Continue processing other files
		}

		rp.logger.Info("Successfully parsed file", zap.String("path", filePath))
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to process repository %s: %w", repo.Name, err)
	}

	rp.logger.Info("Completed processing repository", zap.String("name", repo.Name))
	return nil
}

func (rp *RepoProcessor) ProcessAllRepositories(ctx context.Context, postProcessor *PostProcessor) error {
	rp.logger.Info("Starting to process all repositories", zap.Int("count", len(rp.config.Source.Repositories)))
	executorPool := util.NewExecutorPool(5, 100, func(task any) {
		repo := task.(*config.Repository)
		err := rp.ProcessRepository(ctx, repo)
		if err != nil {
			rp.logger.Error("Failed to process repository", zap.String("name", repo.Name), zap.Error(err))
			return
		}

		err = postProcessor.PostProcessRepository(ctx, repo)
		if err != nil {
			rp.logger.Error("Failed to post-process repository", zap.String("name", repo.Name), zap.Error(err))
			return
		}
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
			/*err := rp.ProcessRepository(ctx, &repo)
			if err != nil {
				rp.logger.Error("Failed to process repository", zap.String("name", repo.Name), zap.Error(err))
				// Continue processing other repositories even if one fails
				continue
			}
			err = postProcessor.PostProcessRepository(ctx, &repo)
			if err != nil {
				rp.logger.Error("Failed to post-process repository", zap.String("name", repo.Name), zap.Error(err))
				// Continue processing other repositories even if one fails
				continue
			}*/
			executorPool.Submit(&repo)
		}
	}

	rp.logger.Info("Completed processing all repositories")
	return nil
}

func (rp *RepoProcessor) generateFileID(ctx context.Context, filePath string) int32 {
	fileID, err := rp.codeGraph.GetOrCreateNextFileID(ctx)
	if err != nil {
		rp.logger.Error("Failed to generate file ID", zap.String("path", filePath), zap.Error(err))
		return 0
	}
	return fileID
	// Simple hash function to generate a unique file ID
	/*hash := int32(0)
	for _, char := range filePath {
		hash = hash*31 + int32(char)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
	*/
}
