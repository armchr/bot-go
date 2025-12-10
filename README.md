# Bot-Go

A GoLang service for analyzing source code repositories using multiple approaches:
- **Language Server Protocol (LSP)**: Real-time code intelligence via language servers (gopls, pylsp, typescript-language-server)
- **Tree-sitter parsing**: Direct AST parsing into graph database representation
- **Graph database storage**: Code structure stored as a graph (Neo4j)
- **Hierarchical code chunking**: Vector embeddings for semantic code search (Qdrant + Ollama)
- **MCP server**: Model Context Protocol server for AI assistants

**Supported languages**: Go, Python, Java, JavaScript, TypeScript

## Architecture Overview

Bot-Go processes repositories in three complementary ways:

1. **CodeGraph** (Optional): Tree-sitter parses files → AST nodes → Graph database → LSP enriches relationships
2. **LSP Analysis**: Direct language server queries for call hierarchies, definitions, and symbols
3. **Vector Search** (Optional): Hierarchical code chunks (file → class → function) with embeddings for similarity search

## Quick Start

### Prerequisites

- Go 1.23+
- Language servers (optional, for LSP features):
  - `gopls` for Go
  - `python-lsp-server` for Python
  - `typescript-language-server` for TypeScript/JavaScript
- Docker (optional, for containerized deployment)
- Qdrant (optional, for vector search): `docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant`
- Ollama (optional, for embeddings): Install from [ollama.ai](https://ollama.ai) and pull a model: `ollama pull qwen3-embedding:0.6b`

### Installation

```bash
# Clone repository
git clone <repository-url>
cd bot-go

# Install dependencies
make deps

# Optional: Install language servers
make install-lsp-servers

# Build binary
make build
```

### Configuration

Bot-Go uses two configuration files:

#### 1. `app.yaml` - Application Settings

```yaml
# Server configuration
mcp:
  host: "localhost"
  port: 8282              # MCP server port
app:
  port: 8181              # REST API port
  codegraph: false        # Enable/disable CodeGraph processing
  gopls: "${BOT_GO_PATH}/scripts/gopls.sh"      # Path to gopls wrapper
  python: "${BOT_GO_PATH}/scripts/pylsp.sh"     # Path to pylsp wrapper
  num_file_threads: 2     # Concurrent file processing threads

# Graph database
neo4j:
  uri: "bolt://localhost:7687"
  username: ""
  password: ""

# Vector search (optional)
qdrant:
  host: "localhost"
  port: 6334              # gRPC port
  apikey: ""
ollama:
  url: "http://localhost:11434"
  apikey: ""
  model: "qwen3-embedding:0.6b"
  dimension: 1024         # Must match model's output dimension

# Chunking configuration
chunking:
  min_conditional_lines: 8  # Minimum lines for separate conditional chunks
  min_loop_lines: 8         # Minimum lines for separate loop chunks
```

**Environment variable expansion**: Use `${VAR_NAME}` for paths. Set `BOT_GO_PATH` to your installation directory.

#### 2. `source.yaml` - Repository Definitions

```yaml
source:
  repositories:
    # Basic configuration
    - name: "my-go-project"
      path: "/path/to/go/project"
      language: "go"
      disabled: false

    # Python project
    - name: "ml-service"
      path: "/path/to/python/project"
      language: "python"
      disabled: false

    # Skip other languages (useful for polyglot repos)
    - name: "backend-only"
      path: "/path/to/fullstack/app"
      language: "go"
      skip_other_languages: true  # Only process .go files
      disabled: false

    # Test mode with specific file
    - name: "test-repo"
      path: "/path/to/test/repo"
      test: "main.go"
      language: "go"
      disabled: true
```

**Configuration options**:
- `name`: Identifier used in API calls (also default Qdrant collection name)
- `path`: Absolute path to repository
- `language`: `go`, `python`, `java`, `javascript`, or `typescript`
- `skip_other_languages`: Only process files matching `language` (default: false)
- `disabled`: Skip this repository (default: false)
- `test`: Process only this specific file (for testing)

### Running Locally

```bash
# Run with default config paths
make run

# Or with custom paths
go run cmd/main.go -app=config/app.yaml -source=config/source.yaml

# Run in test mode (tests LSP client initialization)
go run cmd/main.go -app=config/app.yaml -source=config/source.yaml -test

# Check health
curl http://localhost:8181/api/v1/health
```

### CLI Index Building

Bot-Go can be run in CLI mode to build indexes for repositories without starting the server. This is useful for batch processing, CI/CD pipelines, and testing.

```bash
# Build index for a single repository
./bin/bot-go -app=config/app.yaml -source=config/source.yaml --build-index=my-repo

# Build index for multiple repositories
./bin/bot-go -app=config/app.yaml -source=config/source.yaml \
    --build-index=repo1 --build-index=repo2

# Build index from git HEAD (faster, reads from git object store)
./bin/bot-go -app=config/app.yaml -source=config/source.yaml \
    --build-index=my-repo --head

# Using make shortcuts
make build-index REPO=my-repo
make build-index-head REPO=my-repo
```

**CLI Options for `--build-index` mode:**

| Option | Description |
|--------|-------------|
| `--build-index=<repo>` | Repository name to build index for (can be specified multiple times) |
| `--head` | Read files from git HEAD instead of working directory (faster for clean repos) |
| `--test-dump=<path>` | Dump the code graph to a file after processing (for testing/debugging) |
| `--clean` | Clean up all DB entries after processing (MySQL, Neo4j, Qdrant) |

#### Test Dump (`--test-dump`)

Dumps the complete code graph to a text file after all processing stages complete. Useful for testing and debugging the code graph structure.

```bash
# Build index and dump the resulting code graph
./bin/bot-go -app=config/app.yaml -source=config/source.yaml \
    --build-index=my-repo --test-dump=/tmp/graph-dump.txt
```

The dump file contains:
- All FileScopes sorted alphabetically by path
- All nodes within each file (functions, classes, variables, etc.) with their IDs, names, ranges, and metadata
- All relationships between nodes in the format `(fromID) -[TYPE]-> (toID)`
- Node and relationship counts per file

#### Cleanup (`--clean`)

Removes all data for the specified repositories from all databases after processing. This runs **after** test-dump if both are specified.

```bash
# Build index, then clean up all data
./bin/bot-go -app=config/app.yaml -source=config/source.yaml \
    --build-index=my-repo --clean

# Build index, dump for inspection, then clean up
./bin/bot-go -app=config/app.yaml -source=config/source.yaml \
    --build-index=my-repo --test-dump=/tmp/dump.txt --clean
```

**Cleanup targets:**
- **Neo4j**: Deletes all FileScope nodes and their descendants (functions, classes, etc.) for the repository
- **Qdrant**: Deletes the vector collection for the repository
- **MySQL**: Drops the file_versions table for the repository

**Execution order:**
1. Index building (process all files through enabled processors)
2. Test dump (if `--test-dump` specified)
3. Cleanup (if `--clean` specified)

### Running with Docker

```bash
# Build image
make docker-build

# Run in foreground (interactive)
make docker-run

# Run in background (detached)
make docker-run-detached

# View logs
make docker-logs

# Stop container
make docker-stop
```

**Docker notes**:
- Exposes ports 8181 (REST API) and 8282 (MCP server)
- Mounts `config/` directory for configuration
- Mounts `data/` for database persistence
- Mounts `logs/` for application logs
- Includes file descriptor limit increase for large repos

### Docker Compose (with Memgraph)

```bash
# Start services (bot-go + Memgraph graph database)
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

The compose setup includes:
- **bot-go**: Application container
- **memgraph**: Neo4j-compatible graph database
- Health checks and automatic restart

## API Endpoints

All endpoints use JSON and are available at `http://localhost:8181/api/v1/`.

### Health Check

```bash
GET /api/v1/health
```

**Response**:
```json
{"status": "healthy"}
```

### Process Repository (LSP)

```bash
POST /api/v1/processRepo
Content-Type: application/json

{
  "repo_name": "my-go-project"
}
```

Processes repository using LSP to extract files and functions.

**Parameters**:
- `repo_name` (required): Repository name from `source.yaml`

**Note**: Current implementation returns minimal data. Used for initialization.

### Get Function Dependencies

```bash
POST /api/v1/functionDependencies
Content-Type: application/json

{
  "repo_name": "my-go-project",
  "relative_path": "cmd/main.go",
  "function_name": "main",
  "depth": 2
}
```

Returns function call dependencies using LSP call hierarchy.

**Parameters**:
- `repo_name` (required): Repository name
- `relative_path` (required): File path relative to repo root
- `function_name` (required): Function to analyze
- `depth` (optional): Traversal depth (default: 2)

**Response** (example):
```json
{
  "repo_name": "my-go-project",
  "file_path": "cmd/main.go",
  "function_name": "main",
  "dependencies": [
    {
      "name": "processConfig",
      "call_locations": [
        {
          "uri": "file:///path/to/project/cmd/main.go",
          "range": {
            "start": {"line": 15, "character": 2},
            "end": {"line": 15, "character": 15}
          }
        }
      ],
      "definition": {
        "name": "processConfig",
        "location": {
          "uri": "file:///path/to/project/internal/config/config.go",
          "range": {
            "start": {"line": 10, "character": 0},
            "end": {"line": 30, "character": 1}
          }
        },
        "is_external": false,
        "params": "(cfg *Config)",
        "returns": "error"
      }
    }
  ]
}
```

### Process Directory for Code Chunking

**Requires Qdrant and Ollama to be configured in `app.yaml`**

```bash
POST /api/v1/processDirectory
Content-Type: application/json

{
  "repo_name": "my-go-project",
  "collection_name": "my-collection"
}
```

Chunks code hierarchically (file → class → function → block) and stores embeddings in Qdrant.

**Parameters**:
- `repo_name` (required): Repository name from `source.yaml`
- `collection_name` (optional): Qdrant collection name (defaults to `repo_name`)

**Response**:
```json
{
  "repo_name": "my-go-project",
  "collection_name": "my-collection",
  "total_chunks": 1234,
  "success": true,
  "message": "Directory processed successfully"
}
```

### Search Similar Code

**Requires repository to be processed with `/processDirectory` first**

```bash
POST /api/v1/searchSimilarCode
Content-Type: application/json

{
  "repo_name": "my-go-project",
  "collection_name": "my-collection",
  "code_snippet": "func handleRequest(w http.ResponseWriter, r *http.Request) {\n  // handler logic\n}",
  "language": "go",
  "limit": 10,
  "include_code": true
}
```

Searches for similar code chunks using semantic similarity.

**Parameters**:
- `repo_name` (required): Repository name
- `collection_name` (optional): Collection to search (defaults to `repo_name`)
- `code_snippet` (required): Code snippet to find matches for
- `language` (required): `go`, `python`, `java`, `javascript`, or `typescript`
- `limit` (optional): Max results (default: 10)
- `include_code` (optional): Include actual code content (default: false)

**How it works**:
1. Input snippet is **parsed and chunked** (may produce multiple chunks if it contains multiple functions/classes)
2. **Each chunk is embedded separately** and searches independently
3. Results are **merged and deduplicated** (highest similarity score wins)
4. Each result includes `query_chunk_index` indicating which input chunk matched best
5. Results are **sorted by score** (descending) and limited to `limit`

**Response** (example):
```json
{
  "repo_name": "my-go-project",
  "collection_name": "my-collection",
  "query": {
    "code_snippet": "func handleRequest...",
    "language": "go",
    "chunks_found": 1,
    "chunks": [
      {
        "id": "query-chunk-0",
        "chunk_type": "function",
        "level": 3,
        "content": "func handleRequest(w http.ResponseWriter, r *http.Request) { ... }",
        "language": "go",
        "file_path": "query.snippet",
        "start_line": 0,
        "end_line": 3,
        "name": "handleRequest",
        "signature": "func handleRequest(w http.ResponseWriter, r *http.Request)"
      }
    ]
  },
  "results": [
    {
      "chunk": {
        "id": "abc123",
        "chunk_type": "function",
        "level": 3,
        "parent_id": "parent123",
        "content": "func handleHTTPRequest(w http.ResponseWriter, r *http.Request) { ... }",
        "language": "go",
        "file_path": "/path/to/project/internal/handler/http.go",
        "start_line": 45,
        "end_line": 60,
        "name": "handleHTTPRequest",
        "signature": "func handleHTTPRequest(w http.ResponseWriter, r *http.Request)",
        "docstring": "handleHTTPRequest processes incoming HTTP requests"
      },
      "score": 0.92,
      "query_chunk_index": 0,
      "code": "func handleHTTPRequest(w http.ResponseWriter, r *http.Request) {\n  log.Printf(\"Request: %s %s\", r.Method, r.URL.Path)\n  // implementation\n}"
    }
  ],
  "success": true,
  "message": "Search completed successfully"
}
```

**Response fields**:
- `query.chunks[]`: Array of parsed input chunks (indexed from 0)
- `query.chunks_found`: Number of chunks generated from input
- `results[].chunk`: Metadata about matched code chunk
- `results[].score`: Similarity score (0.0-1.0, higher = more similar)
- `results[].query_chunk_index`: Index of input chunk that matched (reference to `query.chunks[index]`)
- `results[].code`: Actual code content (only if `include_code: true`)

## MCP Server

Bot-Go includes a Model Context Protocol (MCP) server running on port 8282 (configurable via `mcp.port` in `app.yaml`).

**Available tools**:
- `getCallGraph`: Get functions called by a target function (dependencies)
- `getCallerGraph`: Get functions that call a target function (reverse dependencies)

Both tools return hierarchical XML-style output with hover information and source locations.

See [MCP documentation](https://modelcontextprotocol.io/) for integration details.

## Testing

```bash
# Run all tests
make test

# Or directly
go test ./...

# Run specific package tests
go test ./internal/service -v

# Test LSP client initialization
go run cmd/main.go -app=config/app.yaml -source=config/source.yaml -test
```

## Development

### Project Structure

```
bot-go/
├── cmd/                        # Entry points
│   ├── main.go                 # Main service
│   ├── chunk_test.go           # Chunking test utility
│   └── run_eval.go             # Evaluation runner
├── internal/
│   ├── config/                 # Configuration loading
│   ├── controller/             # Business logic
│   │   ├── repo_controller.go  # API controller
│   │   ├── repo_processor.go   # CodeGraph processing
│   │   └── post_process.go     # LSP enrichment
│   ├── handler/                # HTTP handlers
│   ├── model/                  # Data models
│   │   └── code_chunk.go       # CodeChunk model
│   ├── parse/                  # Tree-sitter parsing
│   │   ├── file_parser.go      # File parser factory
│   │   ├── chunk_visitor.go    # Chunking visitor
│   │   └── *_visitor.go        # Language-specific visitors
│   ├── service/                # Core services
│   │   ├── repo_service.go     # LSP repository service
│   │   ├── code_graph.go       # Graph database API
│   │   ├── graph_db.go         # Graph DB interface
│   │   ├── neo4j_db.go         # Neo4j implementation
│   │   ├── vector_db.go        # Vector DB interface
│   │   ├── qdrant_db.go        # Qdrant implementation
│   │   ├── embedding.go        # Embedding interface
│   │   ├── ollama_embedding.go # Ollama implementation
│   │   └── code_chunk_service.go # Chunking service
│   └── util/                   # Utilities
├── pkg/
│   ├── lsp/                    # LSP clients
│   │   ├── base/               # Base client & models
│   │   ├── golang.go           # Go LSP client
│   │   ├── python.go           # Python LSP client
│   │   └── typescript.go       # TypeScript LSP client
│   └── mcp/                    # MCP server
├── config/                     # Configuration files
│   ├── app.yaml                # Application config
│   ├── source.yaml             # Repository config (gitignored)
│   └── source.example.yaml     # Example repository config
├── scripts/                    # Helper scripts
│   ├── gopls.sh                # gopls wrapper
│   └── pylsp.sh                # pylsp wrapper
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

### Adding New Language Support

1. Add language server initialization in `pkg/lsp/langserver.go`
2. Create new LSP client in `pkg/lsp/` (extend `BaseClient`)
3. Add tree-sitter grammar to `go.mod`
4. Create visitor in `internal/parse/` (implement `SyntaxTreeVisitor`)
5. Update `FileParser.DetectLanguage()` and `GetLanguageParser()`
6. Add test repository to `source.yaml`

### Key Design Patterns

**Graph Database Abstraction**: Neo4j supported via unified interface with Cypher queries

**LSP Client Pattern**: Base client handles JSON-RPC, language-specific clients extend with custom initialization

**Visitor Pattern**: Tree-sitter AST traversal via language-specific visitors

**Concurrent Processing**: `ExecutorPool` provides worker pool for parallel repository processing

**Hierarchical Chunking**: Four-level hierarchy (File → Class → Function → Block) with parent references

## Dependencies

- **github.com/gin-gonic/gin** - HTTP routing
- **go.uber.org/zap** - Structured logging
- **github.com/tree-sitter/go-tree-sitter** - AST parsing
- **github.com/neo4j/neo4j-go-driver/v5** - Neo4j client
- **github.com/qdrant/go-client** - Qdrant vector DB
- **github.com/modelcontextprotocol/go-sdk** - MCP protocol
- **gopkg.in/yaml.v2** - YAML configuration

All dependencies are open source (MIT/Apache/BSD licenses).

## Troubleshooting

### Common Issues

**Repository not found**:
- Verify `repo_name` matches `source.yaml`
- Check path exists and is accessible

**Language server errors**:
- Ensure language servers are installed (`make install-lsp-servers`)
- Verify paths in `app.yaml` are correct
- Check `BOT_GO_PATH` environment variable is set

**Port already in use**:
- Change port in `app.yaml`
- Or use environment variable override (future feature)

**Qdrant connection failed**:
- Verify Qdrant is running: `curl http://localhost:6333/health`
- Check host/port in `app.yaml`

**Ollama embedding errors**:
- Ensure Ollama is running: `curl http://localhost:11434/api/tags`
- Pull embedding model: `ollama pull qwen3-embedding:0.6b`
- Verify model name and dimension match in `app.yaml`

**Docker file descriptor limit**:
- Already configured in Dockerfile with ulimit increase
- For Docker run: `--ulimit nofile=65536:65536`

### Logs

Logs are written to:
- **stdout**: Console output
- **all.log**: File in working directory (or `/app/logs/` in Docker)

Log level is set to **Debug** by default. Uses structured JSON logging (Zap).

## Contributing

Contributions are welcome! Please ensure:
- Tests pass (`make test`)
- Code follows Go conventions
- Documentation is updated

## License

This project is intended for educational and development purposes.
