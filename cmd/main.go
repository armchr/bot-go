package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"bot-go/internal/config"
	"bot-go/internal/controller"
	"bot-go/internal/handler"
	"bot-go/internal/service"
	"bot-go/internal/service/codegraph"
	"bot-go/internal/service/ngram"
	"bot-go/internal/service/vector"
	"bot-go/pkg/lsp"
	"bot-go/pkg/mcp"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// stringSliceFlag is a custom flag type that allows multiple values
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var sourceConfigPath = flag.String("source", "source.yaml", "Path to source configuration file")
	var appConfigPath = flag.String("app", "app.yaml", "Path to app configuration file")
	var workDir = flag.String("workdir", "", "Working directory to store files")
	//var port = flag.String("port", "8080", "Server port")
	var test = flag.Bool("test", false, "Run in test mode")
	var buildIndex stringSliceFlag
	flag.Var(&buildIndex, "build-index", "Repository name to build index for (can be specified multiple times)")
	flag.Parse()

	//logger, err := zap.NewProduction()
	cfgZap := zap.NewProductionConfig()
	cfgZap.Level.SetLevel(zapcore.DebugLevel)
	cfgZap.OutputPaths = []string{"stdout", "all.log"}
	logger, err := cfgZap.Build()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	defer logger.Sync()

	cfg, err := config.LoadConfig(*appConfigPath, *sourceConfigPath)
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Override workdir from command line if provided
	if *workDir != "" {
		cfg.App.WorkDir = *workDir
	}

	logger.Info("Configuration loaded successfully", zap.Any("config", cfg))

	if test != nil && *test {
		logger.Info("Running in test mode")
		LSPTest(cfg, logger)
		return
	}

	// Check if we're in CLI mode (build-index specified)
	if len(buildIndex) > 0 {
		logger.Info("Running in CLI mode - build-index")
		BuildIndexCommand(cfg, logger, buildIndex)
		return
	}

	repoService := service.NewRepoService(cfg, logger)
	CodeGraphEntry(cfg, logger, repoService)

	// Initialize CodeChunkService if Qdrant and Ollama are configured
	var chunkService *vector.CodeChunkService
	if cfg.Qdrant.Host != "" && cfg.Ollama.URL != "" {
		logger.Info("Initializing code chunk service",
			zap.String("qdrant_host", cfg.Qdrant.Host),
			zap.Int("qdrant_port", cfg.Qdrant.Port),
			zap.String("ollama_url", cfg.Ollama.URL))

		vectorDB, err := vector.NewQdrantDatabase(cfg.Qdrant.Host, cfg.Qdrant.Port, cfg.Qdrant.APIKey, logger)
		if err != nil {
			logger.Warn("Failed to initialize Qdrant database, code chunking will be disabled", zap.Error(err))
		} else {
			embeddingModel, err := vector.NewOllamaEmbedding(vector.OllamaEmbeddingConfig{
				APIURL:    cfg.Ollama.URL,
				APIKey:    cfg.Ollama.APIKey,
				Model:     cfg.Ollama.Model,
				Dimension: cfg.Ollama.Dimension,
			}, logger)
			if err != nil {
				logger.Warn("Failed to initialize Ollama embedding model, code chunking will be disabled", zap.Error(err))
				vectorDB.Close()
			} else {
				// Set default thresholds if not configured
				minConditionalLines := cfg.Chunking.MinConditionalLines
				minLoopLines := cfg.Chunking.MinLoopLines
				if minConditionalLines == 0 {
					minConditionalLines = 5 // default
				}
				if minLoopLines == 0 {
					minLoopLines = 5 // default
				}

				gcThreshold := cfg.App.GCThreshold
				if gcThreshold == 0 {
					gcThreshold = 100 // default
				}

				numFileThreads := cfg.App.NumFileThreads
				if numFileThreads == 0 {
					numFileThreads = 2 // default
				}

				chunkService = vector.NewCodeChunkService(
					vectorDB,
					embeddingModel,
					minConditionalLines,
					minLoopLines,
					gcThreshold,
					numFileThreads,
					logger,
				)
				logger.Info("Code chunk service initialized successfully",
					zap.Int("min_conditional_lines", minConditionalLines),
					zap.Int("min_loop_lines", minLoopLines),
					zap.Int64("gc_threshold", gcThreshold))
			}
		}
	} else {
		logger.Info("Code chunk service disabled (Qdrant or Ollama not configured)")
	}

	// Initialize NGramService
	ngramService, err := ngram.NewNGramService(logger)
	if err != nil {
		logger.Warn("Failed to initialize N-gram service", zap.Error(err))
	} else {
		logger.Info("N-gram service initialized successfully")
	}

	repoController := controller.NewRepoController(repoService, chunkService, ngramService, logger)
	mcpServer := mcp.NewCodeGraphServer(repoService, cfg, logger)

	router := handler.SetupRouter(repoController, mcpServer, logger)

	logger.Info("Starting server", zap.Int("port", cfg.App.Port))
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.App.Port), router); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}

func LSPTest(cfg *config.Config, logger *zap.Logger) {
	logger.Info("Testing LSP client")
	repo, _ := cfg.GetRepository("mcp-server")

	// Initialize the LSP client
	ls, err := lsp.NewLSPLanguageServer(cfg, repo.Language, repo.Path, logger)
	if err != nil {
		logger.Fatal("Failed to create LSP client", zap.Error(err))
	}

	// Create a context for the LSP operations
	ctx := context.Background()

	defer ls.Shutdown(ctx)

	// Initialize the LSP client

	baseClient := ls.(*lsp.TypeScriptLanguageServerClient).BaseClient

	baseClient.TestCommand(ctx)
}

func BuildIndexCommand(cfg *config.Config, logger *zap.Logger, repoNames []string) {
	ctx := context.Background()

	logger.Info("Build index command started",
		zap.Strings("repositories", repoNames),
		zap.Bool("code_graph_enabled", cfg.IndexBuilding.EnableCodeGraph),
		zap.Bool("embeddings_enabled", cfg.IndexBuilding.EnableEmbeddings),
		zap.Bool("ngram_enabled", cfg.IndexBuilding.EnableNgram))

	// Initialize processors based on configuration
	var processors []controller.FileProcessor

	// Initialize CodeGraph processor if enabled
	var codeGraph *codegraph.CodeGraph
	if cfg.IndexBuilding.EnableCodeGraph {
		var err error
		codeGraph, err = codegraph.NewCodeGraph(
			cfg.Neo4j.URI,
			cfg.Neo4j.Username,
			cfg.Neo4j.Password,
			cfg,
			logger,
		)
		if err != nil {
			logger.Fatal("Failed to initialize CodeGraph", zap.Error(err))
			return
		}
		defer codeGraph.Close(ctx)

		// Initialize RepoService for LSP (needed for post-processing)
		repoService := service.NewRepoService(cfg, logger)

		codeGraphProcessor := controller.NewCodeGraphProcessor(cfg, codeGraph, repoService, logger)
		processors = append(processors, codeGraphProcessor)

		logger.Info("CodeGraph processor initialized for index building")
	}

	// Initialize Embedding processor if enabled
	if cfg.IndexBuilding.EnableEmbeddings {
		if cfg.Qdrant.Host == "" || cfg.Ollama.URL == "" {
			logger.Fatal("Embeddings enabled but Qdrant or Ollama not configured")
			return
		}

		vectorDB, err := vector.NewQdrantDatabase(cfg.Qdrant.Host, cfg.Qdrant.Port, cfg.Qdrant.APIKey, logger)
		if err != nil {
			logger.Fatal("Failed to initialize Qdrant database", zap.Error(err))
			return
		}
		defer vectorDB.Close()

		embeddingModel, err := vector.NewOllamaEmbedding(vector.OllamaEmbeddingConfig{
			APIURL:    cfg.Ollama.URL,
			APIKey:    cfg.Ollama.APIKey,
			Model:     cfg.Ollama.Model,
			Dimension: cfg.Ollama.Dimension,
		}, logger)
		if err != nil {
			logger.Fatal("Failed to initialize Ollama embedding model", zap.Error(err))
			return
		}

		// Set default thresholds if not configured
		minConditionalLines := cfg.Chunking.MinConditionalLines
		minLoopLines := cfg.Chunking.MinLoopLines
		if minConditionalLines == 0 {
			minConditionalLines = 5
		}
		if minLoopLines == 0 {
			minLoopLines = 5
		}

		gcThreshold := cfg.App.GCThreshold
		if gcThreshold == 0 {
			gcThreshold = 100
		}

		numFileThreads := cfg.App.NumFileThreads
		if numFileThreads == 0 {
			numFileThreads = 2
		}

		chunkService := vector.NewCodeChunkService(
			vectorDB,
			embeddingModel,
			minConditionalLines,
			minLoopLines,
			gcThreshold,
			numFileThreads,
			logger,
		)

		embeddingProcessor := controller.NewEmbeddingProcessor(chunkService, logger)
		processors = append(processors, embeddingProcessor)

		logger.Info("Embedding processor initialized for index building")
	}

	// Initialize NGram processor if enabled
	if cfg.IndexBuilding.EnableNgram {
		ngramService, err := ngram.NewNGramService(logger)
		if err != nil {
			logger.Fatal("Failed to initialize N-gram service", zap.Error(err))
			return
		}

		n := 3 // trigrams
		override := false
		ngramProcessor := controller.NewNGramProcessor(ngramService, n, override, logger)
		processors = append(processors, ngramProcessor)

		logger.Info("N-gram processor initialized for index building")
	}

	// Create the index builder with all processors
	indexBuilder := controller.NewIndexBuilder(cfg, processors, logger)

	// Process each repository
	for _, repoName := range repoNames {
		logger.Info("Processing repository for index building",
			zap.String("repo_name", repoName))

		// Validate repository exists in config
		repo, err := cfg.GetRepository(repoName)
		if err != nil {
			logger.Error("Repository not found in configuration",
				zap.String("repo_name", repoName),
				zap.Error(err))
			continue
		}

		logger.Info("Building indexes for repository",
			zap.String("repo_name", repo.Name),
			zap.String("path", repo.Path),
			zap.String("language", repo.Language))

		// Build all indexes using the unified index builder
		if err := indexBuilder.BuildIndex(ctx, repo); err != nil {
			logger.Error("Failed to build indexes for repository",
				zap.String("repo_name", repo.Name),
				zap.Error(err))
			continue
		}

		logger.Info("Completed index building for repository",
			zap.String("repo_name", repo.Name))
	}

	logger.Info("Build index command completed")
}

func CodeGraphEntry(cfg *config.Config, logger *zap.Logger, repoService *service.RepoService) {
	if !cfg.App.CodeGraph {
		logger.Info("CodeGraph is disabled in the configuration")
		return
	}
	ctx := context.Background()

	// Initialize CodeGraph service
	codeGraph, err := codegraph.NewCodeGraph(
		cfg.Neo4j.URI,
		cfg.Neo4j.Username,
		cfg.Neo4j.Password,
		cfg,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to initialize CodeGraph", zap.Error(err))
		return
	}
	//defer codeGraph.Close(ctx)

	// Initialize RepoProcessor
	repoProcessor := controller.NewRepoProcessor(cfg, codeGraph, logger)
	repoProcessor.SetRepoService(repoService) // Set repo service for LSP post-processing
	postProcessor := controller.NewPostProcessor(codeGraph, repoService.GetLspService(), logger)

	// Start processing repositories in a goroutine
	go func() {
		logger.Info("Starting repository processing thread")
		err := repoProcessor.ProcessAllRepositories(ctx, postProcessor)

		if err != nil {
			logger.Error("Repository processing failed", zap.Error(err))
		}
		logger.Info("Repository processing thread completed")
	}()
}
