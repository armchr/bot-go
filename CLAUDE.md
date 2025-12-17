# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Bot-Go is a GoLang service that analyzes source code repositories using multiple approaches:
- **Language Server Protocol (LSP)**: Integrates with language servers (gopls, pylsp, typescript-language-server) for real-time code intelligence
- **Tree-sitter parsing**: Direct AST parsing of code files into a graph database representation
- **Graph database storage**: Stores code structure as a graph using Neo4j
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

# Build indexes for repositories (CLI mode)
make build-index REPO=bot-go                    # Build index from disk
make build-index-head REPO=bot-go               # Build index from git HEAD (faster)
./bin/bot-go -app=config/app.yaml -source=config/source.yaml -build-index="repo-name"
./bin/bot-go -app=config/app.yaml -source=config/source.yaml -build-index="repo-name" --head
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
- Database connection (neo4j.uri)
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
   - AST nodes are stored in graph database (Neo4j)
   - `PostProcessor` enriches function call relationships using LSP

3. **LSP Integration**:
   - `RepoService` manages LSP clients for each repository
   - LSP clients (GoLanguageServerClient, PythonLanguageServerClient, TypeScriptLanguageServerClient) communicate via JSON-RPC over stdio
   - Used for: hover info, go-to-definition, document symbols, call hierarchies

### Key Components

**internal/service/graph_db.go**:
- `GraphDatabase` interface abstracts graph database implementations
- Uses Cypher queries
- Implementation in `neo4j_db.go`

**internal/db/mysql.go & file_version.go**:
- MySQL database for tracking file versions and processing status
- `MySQLConnection` manages database lifecycle and ensures database exists
- `FileVersionRepository` manages file version tracking with per-repository tables
- Table naming: Repository names are sanitized (e.g., `bot-go` → `bot_go_file_versions`)
- Each file tracked by: `file_id`, `file_sha` (SHA256), `relative_path`, `ephemeral`, `commit_id`, `status`
- **File versioning**:
  - Files tied to git commits have `commit_id` and `ephemeral=false`
  - Modified/uncommitted files have `ephemeral=true` and no `commit_id`
  - Unique constraint on `(file_sha, relative_path, commit_id)` prevents duplicates
- **Status tracking**: Monitors processing progress through stages:
  - Default: `processing` (when FileID created)
  - Per-processor: `CodeGraph_done`, `Embedding_done`, `NGram_done` (after each processor completes)
  - Final: `done` (when all processors complete)
- **Schema migration**: `EnsureTable()` automatically adds missing columns (e.g., `status`) to existing tables
- Used by `IndexBuilder` to track which files have been processed and their current state

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

**internal/controller/index_builder.go**:
- `IndexBuilder` orchestrates parallel file processing through registered processors
- **File processing pipeline**:
  1. Walk repository directory with `WalkDirTree()` (concurrent, configurable threads)
  2. Skip special files (Dockerfile, vendor/, node_modules/, bin/, etc.) and optionally non-matching languages
  3. Read file content (optimized with `--head` flag to read from git object store)
  4. Create `FileContext` with FileID from MySQL (tracks SHA256, path, commit, ephemeral status)
  5. Process through all registered processors (CodeGraph, Embedding, NGram) sequentially
  6. Update status after each processor completes
- **Git HEAD mode** (`--head` flag):
  - Reads unmodified files from git object store instead of disk (faster)
  - Gracefully skips untracked files with debug logging
  - Tracks which files were read from git vs disk in logs
- **Language filtering**: When `skip_other_languages` enabled, only process files matching repo language (including variants)
- Processors can be selectively enabled via config: `EnableCodeGraph`, `EnableEmbeddings`, `EnableNgram`

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

**Health & Index Building:**
- `GET /api/v1/health` - Health check endpoint
  - Returns: `{"status": "healthy"}`

- `POST /api/v1/buildIndex` - Build indexes for a repository
  - Parameters: `{"repo_name": "string", "use_head": bool}`
  - `repo_name` (required): Repository name from source.yaml
  - `use_head` (optional): Use git HEAD version instead of working directory (default: false)
  - Returns: `{"repo_name": "string", "status": "completed", "message": "string"}`
  - Builds all indexes (CodeGraph, Embeddings, N-gram) using registered processors
  - HTTP equivalent of the `--build-index` CLI command

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

**CodeAPI** (path: `/codeapi/v1`):

A clean, high-level API for querying the code graph. Provides two main interfaces:
- **CodeReader**: Repository-scoped entity queries (files, classes, methods, fields)
- **GraphAnalyzer**: Graph traversals (call graphs, data flow, impact analysis)

**Reader Endpoints:**

- `GET /codeapi/v1/repos` - List all available repositories
  - Returns: `{"repos": ["repo1", "repo2"]}`

- `POST /codeapi/v1/files` - List files in a repository
  - Parameters: `{"repo_name": "string", "limit": int, "offset": int}`
  - Returns: `{"files": [FileInfo...]}`

- `POST /codeapi/v1/classes` - List classes in a repository
  - Parameters: `{"repo_name": "string", "limit": int, "offset": int}`
  - Returns: `{"classes": [ClassInfo...]}`

- `POST /codeapi/v1/methods` - List methods in a repository
  - Parameters: `{"repo_name": "string", "limit": int, "offset": int}`
  - Returns: `{"methods": [MethodInfo...]}`

- `POST /codeapi/v1/functions` - List top-level functions (not class methods)
  - Parameters: `{"repo_name": "string", "limit": int, "offset": int}`
  - Returns: `{"functions": [MethodInfo...]}`

- `POST /codeapi/v1/classes/find` - Find classes by filter
  - Parameters: `{"repo_name": "string", "name": "string", "name_like": "string", "file_path": "string", "limit": int}`
  - Returns: `{"classes": [ClassInfo...]}`

- `POST /codeapi/v1/methods/find` - Find methods by filter
  - Parameters: `{"repo_name": "string", "name": "string", "class_name": "string", "file_path": "string", "limit": int}`
  - Returns: `{"methods": [MethodInfo...]}`

- `POST /codeapi/v1/class` - Get class by ID with optional details
  - Parameters: `{"repo_name": "string", "class_id": int64, "include_methods": bool, "include_fields": bool}`
  - Returns: `{"class": ClassInfo}`

- `POST /codeapi/v1/method` - Get method by ID
  - Parameters: `{"repo_name": "string", "method_id": int64}`
  - Returns: `{"method": MethodInfo}`

- `POST /codeapi/v1/class/methods` - Get methods of a class
  - Parameters: `{"repo_name": "string", "class_id": int64}`
  - Returns: `{"methods": [MethodInfo...]}`

- `POST /codeapi/v1/class/fields` - Get fields of a class
  - Parameters: `{"repo_name": "string", "class_id": int64}`
  - Returns: `{"fields": [FieldInfo...]}`

**Analyzer Endpoints:**

- `POST /codeapi/v1/callgraph` - Get call graph for a function
  - Parameters:
    - `repo_name` (required): Repository name
    - `function_id` (optional): Function node ID
    - `function_name` (optional): Function name (if function_id not provided)
    - `class_name` (optional): Class name to scope search
    - `file_path` (optional): File path to scope search
    - `direction`: "outgoing" (callees), "incoming" (callers), or "both"
    - `max_depth`: Maximum traversal depth (default: 3)
  - Returns: `{"call_graph": CallGraph}`

- `POST /codeapi/v1/callers` - Get functions that call a function
  - Parameters: `{"repo_name": "string", "function_id": int64, "max_depth": int}`
  - Returns: `{"call_graph": CallGraph}`

- `POST /codeapi/v1/callees` - Get functions called by a function
  - Parameters: `{"repo_name": "string", "function_id": int64, "max_depth": int}`
  - Returns: `{"call_graph": CallGraph}`

- `POST /codeapi/v1/data/dependents` - Get nodes that depend on a value
  - Parameters:
    - `repo_name` (required): Repository name
    - `node_id` (optional): Node ID
    - `variable_name` (optional): Variable name (if node_id not provided)
    - `file_path` (optional): File path to scope search
    - `max_depth`: Maximum traversal depth
    - `include_indirect`: Include transitive dependencies
  - Returns: `{"dependency_graph": DependencyGraph}`

- `POST /codeapi/v1/data/sources` - Get nodes that contribute to a value
  - Parameters: `{"repo_name": "string", "node_id": int64, "max_depth": int}`
  - Returns: `{"dependency_graph": DependencyGraph}`

- `POST /codeapi/v1/impact` - Impact analysis for a node
  - Parameters:
    - `repo_name` (required): Repository name
    - `node_id` (optional): Node ID
    - `name` (optional): Entity name (if node_id not provided)
    - `node_type`: "function", "class", "field", or "variable"
    - `file_path` (optional): File path to scope search
    - `max_depth`: Maximum traversal depth
    - `include_call_graph`: Include call graph in impact
    - `include_data_flow`: Include data flow in impact
  - Returns: `{"impact": ImpactResult}`

- `POST /codeapi/v1/inheritance` - Get inheritance tree for a class
  - Parameters: `{"repo_name": "string", "class_id": int64}`
  - Returns: `{"inheritance_tree": InheritanceTree}`

- `POST /codeapi/v1/field/accessors` - Get methods that access a field
  - Parameters:
    - `repo_name` (required): Repository name
    - `field_id` (optional): Field node ID
    - `class_name` (optional): Class name (if field_id not provided)
    - `field_name` (optional): Field name (if field_id not provided)
  - Returns: `{"field_accessors": FieldAccessResult}`

**Raw Cypher Endpoints:**

- `POST /codeapi/v1/cypher` - Execute read-only Cypher query
  - Parameters: `{"query": "MATCH (n) RETURN n LIMIT 10", "params": {"key": "value"}}`
  - Returns: `{"results": [...]}`

- `POST /codeapi/v1/cypher/write` - Execute write Cypher query
  - Parameters: `{"query": "CREATE (n:Label {name: $name})", "params": {"name": "test"}}`
  - Returns: `{"results": [...]}`

**Example Usage:**
```bash
# List all repositories
curl http://localhost:8181/codeapi/v1/repos

# List classes with pagination
curl -X POST http://localhost:8181/codeapi/v1/classes \
  -H "Content-Type: application/json" \
  -d '{"repo_name": "bot-go", "limit": 10, "offset": 0}'

# Get call graph for a function by name
curl -X POST http://localhost:8181/codeapi/v1/callgraph \
  -H "Content-Type: application/json" \
  -d '{"repo_name": "bot-go", "function_name": "ProcessFile", "direction": "both", "max_depth": 3}'

# Find all methods containing "Process" in name
curl -X POST http://localhost:8181/codeapi/v1/methods/find \
  -H "Content-Type: application/json" \
  -d '{"repo_name": "bot-go", "name_like": "Process", "limit": 20}'

# Execute raw Cypher query
curl -X POST http://localhost:8181/codeapi/v1/cypher \
  -H "Content-Type: application/json" \
  -d '{"query": "MATCH (f:Function)-[:CALLS_FUNCTION*1..2]->(target) WHERE f.name = $name RETURN target.name", "params": {"name": "main"}}'
```

**Programmatic Usage (Go):**
```go
import "bot-go/internal/codeapi"

// Initialize
api := codeapi.NewCodeAPI(codeGraph, logger)

// Reader queries
repo := api.Reader().Repo("bot-go")
classes, _ := repo.ListClasses(ctx, 100, 0)
class, _ := repo.FindClassByName(ctx, "CodeGraph")
methods, _ := repo.GetClassMethods(ctx, classID)

// File-scoped queries
file := repo.File("internal/service/code_graph.go")
methods, _ := file.ListMethods(ctx)

// Analyzer queries
callGraph, _ := api.Analyzer().GetCallGraph(ctx, functionID, codeapi.CallGraphOptions{
    Direction: codeapi.DirectionBoth,
    MaxDepth:  3,
})

// Impact analysis
impact, _ := api.Analyzer().GetImpact(ctx, nodeID, codeapi.ImpactOptions{
    IncludeCallGraph: true,
    IncludeDataFlow:  true,
    MaxDepth:         3,
})

// Raw Cypher
results, _ := api.ExecuteCypher(ctx, "MATCH (n:Function) RETURN n.name LIMIT 10", nil)
```

## Important Patterns

### Graph Database Abstraction
The codebase uses Neo4j via a unified interface:
- Neo4j: Production-ready, requires separate server
- Uses Cypher queries
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
- Neo4j: Use browser at http://localhost:7474
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
