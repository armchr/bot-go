# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Bot-Go is a GoLang service that analyzes source code repositories using multiple approaches:
- **Language Server Protocol (LSP)**: Integrates with language servers (gopls, pylsp, typescript-language-server) for real-time code intelligence
- **Tree-sitter parsing**: Direct AST parsing of code files into a graph database representation
- **Graph database storage**: Stores code structure as a graph using either Neo4j or Kuzu
- **MCP server**: Exposes code analysis tools via Model Context Protocol for AI assistants
- **Hierarchical Code Chunking**: Chunks code into hierarchical pieces with vector embeddings for semantic search (NEW)

Supported languages: Go, Python, JavaScript/TypeScript, Java

## Build and Run Commands

### Local Development
```bash
# Install dependencies
make deps

# Build binary
make build

# Run service (requires config files)
make run

# Run with custom parameters
go run cmd/main.go -source=config/source.yaml -app=config/app.yaml -workdir=/path/to/workdir

# Run in test mode (tests LSP client)
go run cmd/main.go -config=source.yaml -test

# Clean build artifacts
make clean
```

### Testing
```bash
# Run all tests
make test
go test ./...

# Run specific test
go test ./internal/service -v

# Test hierarchical code chunking (requires Qdrant and Jina API key)
go run cmd/chunk_test.go -jina-key YOUR_KEY -test all
```

### Docker
```bash
# Build Docker image
make docker-build

# Run single container
make docker-run

# Use Docker Compose (includes Memgraph)
make docker-compose-up
make docker-compose-down
make docker-compose-logs
```

## Configuration Architecture

The service uses two separate YAML configuration files:

### app.yaml - Application settings
- Server ports (app.port, mcp.port)
- CodeGraph enable/disable flag
- Paths to language server executables (gopls, python)
- Database connection (neo4j.uri, kuzu.path)
- Working directory for temporary files

### source.yaml - Repository definitions
- List of repositories to analyze
- Each repo has: name, path, language, disabled flag
- Optional test path for specific file testing

Configuration loading: `config.LoadConfig(appConfigPath, sourceConfigPath)` merges both configs, with source.yaml overriding certain app.yaml settings.

## Architecture

### Core Flow
1. **cmd/main.go** - Entry point that:
   - Loads configuration from two YAML files
   - Initializes logger (Zap) with both stdout and file output
   - Conditionally starts CodeGraph processing (if enabled in config)
   - Sets up REST API (Gin) and MCP server
   - Runs both servers concurrently

2. **CodeGraph Processing** (when enabled):
   - `RepoProcessor` walks repository files and parses them using tree-sitter
   - Visitors (GoVisitor, PythonVisitor, JavaScriptVisitor) convert syntax trees to AST nodes
   - AST nodes are stored in graph database (Neo4j or Kuzu)
   - `PostProcessor` enriches function call relationships using LSP

3. **LSP Integration**:
   - `RepoService` manages LSP clients for each repository
   - LSP clients (GoLanguageServerClient, PythonLanguageServerClient, TypeScriptLanguageServerClient) communicate via JSON-RPC over stdio
   - Used for: hover info, go-to-definition, document symbols, call hierarchies

### Key Components

**internal/service/graph_db.go**:
- `GraphDatabase` interface abstracts Neo4j and Kuzu implementations
- Both databases support Cypher-like queries
- Implementations in `neo4j_db.go` and `kuzu_db.go`

**internal/service/code_graph.go**:
- High-level API for creating/reading code graph nodes and relationships
- Node types: FileScope, Function, Class, Variable, Block, Expression, FunctionCall, etc.
- Relationship types: CONTAINS, CALLS, HAS_FIELD, INHERITS, etc.
- Uses `writeNode()` and `readNodes()` internally with Cypher queries

**pkg/lsp/**:
- Language server clients implement `base.LSPClient` interface
- Each language client extends `BaseClient` which handles JSON-RPC communication
- Language servers run as subprocesses, communicate via stdin/stdout

**internal/parse/**:
- `FileParser` detects language and creates appropriate visitor
- Language-specific visitors (GoVisitor, PythonVisitor, JavaScriptVisitor) traverse tree-sitter AST
- `TranslateFromSyntaxTree` manages node/scope stack and generates unique IDs

**pkg/mcp/server.go**:
- Implements Model Context Protocol server with two tools:
  - `getCallGraph`: Returns functions called by a target function (dependencies)
  - `getCallerGraph`: Returns functions that call a target function (reverse dependencies)
- Tools return hierarchical XML-style output with hover information
- MCP server runs on separate port (configured in mcp.port)

**internal/controller/repo_processor.go**:
- `ProcessAllRepositories()` uses `ExecutorPool` for concurrent processing
- Walks filesystem using `filepath.Walk()`
- Each file is parsed and traversed to create graph nodes

**internal/controller/post_process.go**:
- `PostProcessRepository()` runs after tree-sitter parsing
- Uses LSP to find actual function definitions for unresolved function calls
- Creates CALLS relationships between function call nodes and function definition nodes

### API Endpoints

REST API (port from app.yaml, default 8181):

**Health & Repository Processing:**
- `GET /api/v1/health` - Health check endpoint
  - Returns: `{"status": "healthy"}`

- `POST /api/v1/processRepo` - Process repository using LSP
  - Parameters: `{"repo_name": "string"}`
  - Returns: Files and functions extracted from the repository
  - Note: Currently returns null (implementation commented out)

**Function Analysis:**
- `POST /api/v1/functionDependencies` - Get function call dependencies using LSP
  - Parameters:
    - `repo_name` (required): Repository name from source.yaml
    - `relative_path` (required): File path relative to repo root
    - `function_name` (required): Name of the function to analyze
    - `depth` (optional): Depth of dependency traversal (default: 2)
  - Returns: Call graph with function dependencies, call locations, and definitions
  - Uses LSP's call hierarchy feature to trace function calls

**Code Chunking & Vector Search** (requires Qdrant + Ollama):
- `POST /api/v1/processDirectory` - Chunk and index a repository's code
  - Parameters:
    - `repo_name` (required): Repository name from source.yaml
    - `collection_name` (optional): Qdrant collection name (defaults to repo_name)
  - Returns: Total chunks created and success status
  - Creates hierarchical code chunks (file → class → function → block) with embeddings

- `POST /api/v1/searchSimilarCode` - Search for similar code using a snippet
  - Parameters:
    - `repo_name` (required): Repository name
    - `collection_name` (optional): Collection to search (defaults to repo_name)
    - `code_snippet` (required): Code snippet to find similar matches for
    - `language` (required): One of: `go`, `python`, `java`, `javascript`, `typescript`
    - `limit` (optional): Max results (default: 10)
    - `include_code` (optional): Include actual code content (default: false)
  - Returns: Query info with parsed chunks, similar code chunks with similarity scores, query chunk index, and optional code content
  - **Multi-chunk query processing**:
    1. Input snippet is parsed with tree-sitter and may generate multiple chunks (e.g., 2 functions → 2 query chunks)
    2. Each query chunk is embedded separately and searches independently
    3. Results are aggregated and deduplicated (keeping highest score per result chunk)
    4. Each result includes `query_chunk_index` (0-based) indicating which input chunk matched best
    5. Response includes:
       - `query.chunks[]`: Array of parsed input chunks (use index to map to results)
       - `query.chunks_found`: Total number of query chunks
       - `results[]`: Matched chunks with `query_chunk_index` referencing `query.chunks[]`

MCP Server (port from app.yaml mcp.port, default 8282):
- HTTP transport for Model Context Protocol
- Exposes tools for AI assistants:
  - `getCallGraph`: Returns functions called by a target function (dependencies)
  - `getCallerGraph`: Returns functions that call a target function (reverse dependencies)
- Tools return hierarchical XML-style output with hover information
- Runs on separate goroutine/port from main REST API

## Important Patterns

### Graph Database Abstraction
The codebase supports both Neo4j and Kuzu via a unified interface. Key differences:
- Neo4j: Production-ready, requires separate server
- Kuzu: Embedded database, can use `:memory:` or file path
- Both use Cypher queries, with slight dialect differences
- Type conversions (int32/int64) are handled in `convertToInt64()` and `convertToInt32()`

### LSP Client Pattern
- Base client handles JSON-RPC protocol
- Language-specific clients implement custom initialization
- Workspace files are tracked and opened incrementally
- Thread-safe with mutexes for request ID generation

### AST Node IDs
- File IDs: Allocated using `GetOrCreateNextFileID()` from FileNumber node in DB
- Node IDs: Generated deterministically by `TranslateFromSyntaxTree.GenerateID()`
- Range encoding: Serialized as string format "(line,char)-(line,char)"

### Concurrent Processing
- `ExecutorPool` in internal/util/ provides worker pool pattern
- Used for processing multiple repositories in parallel
- Configurable worker count and queue size

## Adding New Language Support

1. Add language to `pkg/lsp/langserver.go` NewLSPLanguageServer switch
2. Create new LSP client in `pkg/lsp/` extending `BaseClient`
3. Add tree-sitter grammar dependency in go.mod
4. Create visitor in `internal/parse/` implementing `SyntaxTreeVisitor`
5. Update `FileParser.DetectLanguage()` and `GetLanguageParser()`
6. Test with a sample repository in source.yaml

## Key Dependencies

- **github.com/gin-gonic/gin**: HTTP routing
- **go.uber.org/zap**: Structured logging
- **github.com/tree-sitter/go-tree-sitter**: AST parsing
- **github.com/neo4j/neo4j-go-driver/v5**: Neo4j client
- **github.com/kuzudb/go-kuzu**: Kuzu embedded graph DB
- **github.com/modelcontextprotocol/go-sdk**: MCP protocol implementation
- **gopkg.in/yaml.v2**: YAML configuration

## Common Debugging Tasks

Check logs:
- Application writes to `all.log` in working directory
- Log level is set to Debug in main.go (zapcore.DebugLevel)

Verify LSP connection:
- Run with `-test` flag to test LSP client initialization
- Check configured paths in app.yaml (gopls, python)

Inspect graph database:
- Neo4j: Use browser at http://localhost:7687
- Kuzu: Query via code, or use CLI tool
- Node labels: FileScope, Function, Class, Variable, Block, etc.

Test MCP tools:
- MCP server runs on separate port (check mcp.port in config)
- Use MCP inspector or HTTP client to call tools
- Tools require repo_name, file_path, and function_name parameters

## Hierarchical Code Chunking & Vector Search

**NEW FEATURE**: The codebase now includes hierarchical code chunking with vector embeddings for semantic code search.

### Overview
- Parses code using tree-sitter into hierarchical chunks (file → class → function → block)
- Generates embeddings using Jina AI's code-specific model
- Stores in Qdrant vector database for similarity search
- Supports Go, Python, Java, JavaScript, TypeScript

### Key Components

**Abstractions** (easy to swap implementations):
- `VectorDatabase` interface → Qdrant implementation (can replace with Weaviate, Pinecone, etc.)
- `EmbeddingModel` interface → Jina AI implementation (can replace with OpenAI, Cohere, etc.)

**Core Files**:
- `internal/model/code_chunk.go` - CodeChunk model with metadata
- `internal/service/vector_db.go` - Vector DB interface
- `internal/service/qdrant_db.go` - Qdrant implementation
- `internal/service/embedding.go` - Embedding model interface
- `internal/service/jina_embedding.go` - Jina AI implementation
- `internal/parse/chunk_visitor.go` - Tree-sitter chunking visitor
- `internal/service/code_chunk_service.go` - Orchestration service
- `cmd/chunk_test.go` - Test entry point

### Usage

1. **Start Qdrant**:
```bash
docker run -p 6333:6333 qdrant/qdrant
```

2. **Run chunking test**:
```bash
go run cmd/chunk_test.go \
  -jina-key YOUR_JINA_API_KEY \
  -test all
```

3. **Process a directory**:
```bash
go run cmd/chunk_test.go \
  -jina-key YOUR_KEY \
  -test directory \
  -dir /path/to/code
```

### Chunk Hierarchy
- **Level 1**: File/Module (entire file)
- **Level 2**: Class/Struct (with methods)
- **Level 3**: Function/Method (with signature, docstring)
- **Level 4**: Block (conditionals, loops - future)

Each chunk stores:
- Source code, language, file path, line numbers
- Chunk type, name, signature, docstring
- Parent ID (for hierarchy), context (module, class names)
- Vector embedding (768D for Jina code model)

### Programmatic Usage
```go
// Initialize services
vectorDB, _ := service.NewQdrantDatabase("localhost", 6333, "", logger)
embedding, _ := service.NewJinaEmbedding(service.JinaEmbeddingConfig{
    APIKey: "key", Model: service.JinaCodeModel,
}, logger)
chunkService := service.NewCodeChunkService(vectorDB, embedding, logger)

// Process code
chunkService.ProcessFile(ctx, "main.go", "go", "my-collection")

// Search
chunks, scores, _ := chunkService.SearchSimilarCode(
    ctx, "my-collection", "HTTP request handler", 10, nil,
)
```

See `CODE_CHUNKING.md` for complete documentation.
