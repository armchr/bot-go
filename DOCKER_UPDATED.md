# Docker Guide for Bot-Go

## Overview

Bot-Go can be run as a Docker container, exposing HTTP APIs for code analysis, semantic search, and code intelligence.

## Quick Start

```bash
# Build the image
make docker-build

# Run interactively (foreground)
make docker-run

# Run in background (detached)
make docker-run-detached

# Stop the container
make docker-stop

# View logs
make docker-logs
```

## Docker Commands

### Build

**Build with default tag (latest):**
```bash
make docker-build
# Creates: bot:latest
```

**Build with specific version:**
```bash
make docker-build VERSION=v1.2.3
# Creates: bot:v1.2.3
```

### Run

**1. Interactive mode (foreground, auto-remove on exit):**
```bash
make docker-run
```

Features:
- Runs in foreground with `-it`
- Automatically removed on exit with `--rm`
- Exposes ports: 8181 (API), 8282 (MCP)
- Mounts config files as read-only
- Mounts data and logs directories

**2. Detached mode (background service):**
```bash
make docker-run-detached
```

Features:
- Runs in background with `-d`
- Named container: `bot-go`
- Use `make docker-logs` to view logs
- Use `make docker-stop` to stop

**3. With custom working directory:**
```bash
make docker-run-with-workdir WORKDIR=/path/to/your/workdir
```

Features:
- Mounts custom working directory
- Passes `-workdir=/app/workdir` to bot-go
- Useful for processing specific repositories

### Management

**Stop container:**
```bash
make docker-stop
```

**View logs (follow mode):**
```bash
make docker-logs
```

**View logs (last 100 lines):**
```bash
docker logs --tail 100 bot-go
```

## Push to Docker Hub

**Push to armchr/bot:**
```bash
make docker-push
# Pushes:
#   - armchr/bot:latest
#   - armchr/bot:<VERSION>
```

**Build and push in one command:**
```bash
make docker-release
# Builds locally, then pushes to Docker Hub
```

**With specific version:**
```bash
make docker-push VERSION=v1.2.3
# Pushes:
#   - armchr/bot:v1.2.3
#   - armchr/bot:latest
```

## Command-Line Parameters

Bot-Go accepts the following parameters:

| Parameter | Description | Default | Docker Mount |
|-----------|-------------|---------|--------------|
| `-app` | Path to app config | `app.yaml` | `/app/config/app.yaml` |
| `-source` | Path to source config | `source.yaml` | `/app/config/source.yaml` |
| `-workdir` | Working directory | (none) | `/app/workdir` (optional) |
| `-test` | Run in test mode | `false` | N/A |

## Port Mappings

| Container Port | Host Port | Service |
|----------------|-----------|---------|
| 8181 | 8181 | REST API |
| 8282 | 8282 | MCP Server |

## Volume Mounts

| Host Path | Container Path | Purpose | Mode |
|-----------|----------------|---------|------|
| `config/app.yaml` | `/app/config/app.yaml` | App configuration | ro |
| `config/source.yaml` | `/app/config/source.yaml` | Source repos config | ro |
| `data/` | `/app/data` | Persistent data (Kuzu DB) | rw |
| `logs/` | `/app/logs` | Application logs | rw |
| `(WORKDIR)` | `/app/workdir` | Custom working directory | rw |

## Configuration

### App Configuration (`config/app.yaml`)

Required settings:
```yaml
app:
  port: 8181
  codegraph: true
  gopls: "/path/to/gopls"
  python: "/path/to/pylsp"

mcp:
  host: "0.0.0.0"  # Important: bind to all interfaces in container
  port: 8282

qdrant:
  host: "qdrant"  # Use service name if using docker-compose
  port: 6334
  apikey: ""

ollama:
  url: "http://ollama:11434"  # Use service name if using docker-compose
  model: "qwen3-embedding:0.6b"
  dimension: 1024

chunking:
  min_conditional_lines: 8
  min_loop_lines: 8
```

### Source Configuration (`config/source.yaml`)

Define repositories to analyze:
```yaml
repositories:
  - name: "my-project"
    path: "/app/repos/my-project"
    language: "go"
    disabled: false
```

## API Endpoints

Once running, access the following endpoints:

### Health Check
```bash
curl http://localhost:8181/api/v1/health
```

### Process Directory
```bash
curl -X POST http://localhost:8181/api/v1/processDirectory \
  -H "Content-Type: application/json" \
  -d '{"repo_name": "bot-go", "collection_name": "bot-go"}'
```

### Search Similar Code
```bash
curl -X POST http://localhost:8181/api/v1/searchSimilarCode \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "bot-go",
    "code_snippet": "func main() { ... }",
    "language": "go",
    "limit": 5,
    "include_code": true
  }'
```

## Summary

The Dockerfile and Makefile have been updated with:

### ✅ Dockerfile Changes:
1. Binary name changed from `armchair` to `bot-go`
2. Exposes ports: 8181 (API), 8282 (MCP), 6334 (Qdrant - optional)
3. CMD runs as HTTP service: `./bot-go -app=config/app.yaml -source=config/source.yaml`

### ✅ Makefile Changes:
1. `DOCKER_IMAGE=bot` - Image named "bot"
2. `REGISTRY=armchr` - Default registry for Docker Hub
3. **`make docker-run`** - Interactive mode with all volumes mounted
4. **`make docker-run-detached`** - Background service mode
5. **`make docker-run-with-workdir`** - With custom working directory
6. **`make docker-stop`** - Stop and remove container
7. **`make docker-logs`** - View logs in follow mode
8. **`make docker-push`** - Push to `armchr/bot:latest`
9. **`make docker-release`** - Build and push in one command

All commands properly support the bot-go CLI parameters (`-app`, `-source`, `-workdir`) and run as an HTTP service.
