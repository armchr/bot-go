# Docker Deployment Guide for Armchair

This guide explains how to build, run, and distribute the `armchair` Docker image along with its dependencies.

## Overview

The `armchair` application (formerly bot-go) is containerized as a Docker image that depends on Memgraph (a graph database compatible with Neo4j protocol). The application provides code analysis and repository processing capabilities through a REST API and MCP (Model Context Protocol) server.

The Docker image includes all necessary language servers:
- **gopls** (Go language server) - v0.20.0
- **typescript-language-server** - v5.0.0  
- **pyright** (Python type checker) - v1.1.405
- **python-lsp-server** (Python language server) - v1.13.1

## Architecture

- **armchair**: Main application container running the Go service
- **memgraph**: Graph database container for storing code analysis results
- **Volumes**: Persistent storage for database data and application logs

## Quick Start

### Using Docker Compose (Recommended)

The simplest way to run the full stack:

```bash
# Start all services (armchair + memgraph)
make docker-compose-up

# View logs
make docker-compose-logs

# Stop all services
make docker-compose-down
```

This will:
- Build the armchair image
- Pull the latest Memgraph image
- Start both services with proper networking
- Expose armchair on port 8080
- Expose Memgraph on port 7687 (bolt) and 7444 (web interface)

### Using Docker Directly

If you prefer to run individual containers:

```bash
# Build the armchair image
make docker-build

# Run just the armchair container (requires external Memgraph)
make docker-run
```

## Building the Docker Image

### Local Build

```bash
# Build with default tag (armchair:latest)
make docker-build

# Build with specific version
make docker-build VERSION=v1.2.3
```

### Build for Distribution

```bash
# Build and tag for a registry
make docker-build VERSION=v1.2.3
make docker-tag REGISTRY=your-registry.com VERSION=v1.2.3
```

## Distributing the Docker Image

### Push to Registry

```bash
# Set your registry and push
make docker-push REGISTRY=your-registry.com VERSION=v1.2.3

# Or build and push in one command
make docker-release REGISTRY=your-registry.com VERSION=v1.2.3
```

### Distribution Examples

#### Docker Hub
```bash
make docker-release REGISTRY=docker.io/yourusername VERSION=v1.2.3
```

#### AWS ECR
```bash
# First authenticate with ECR
aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-west-2.amazonaws.com

# Then push
make docker-release REGISTRY=123456789012.dkr.ecr.us-west-2.amazonaws.com VERSION=v1.2.3
```

#### GitHub Container Registry
```bash
# First authenticate
echo $GITHUB_PAT | docker login ghcr.io -u USERNAME --password-stdin

# Then push
make docker-release REGISTRY=ghcr.io/yourusername VERSION=v1.2.3
```

## Configuration

### Environment Variables

The armchair container accepts these environment variables:

- `NEO4J_URI`: Database connection string (default: `bolt://memgraph:7687`)
- `NEO4J_USERNAME`: Database username (default: empty)
- `NEO4J_PASSWORD`: Database password (default: empty)
- `GIN_MODE`: Gin framework mode (`release` for production)

### Configuration File

Mount your `source.yaml` configuration file to `/app/source.yaml`:

```yaml
# Example source.yaml
mcp:
  host: "localhost"
  port: 8282
neo4j:
  uri: "bolt://memgraph:7687"
  username: ""
  password: ""
source:
  repositories:
    - name: "my-repo"
      path: "/path/to/repo"
      language: "go"
```

### Custom Docker Compose

For production deployments, create a custom `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  memgraph:
    image: memgraph/memgraph:latest
    ports:
      - "7687:7687"
    environment:
      - MEMGRAPH_LOG_LEVEL=WARNING
    volumes:
      - memgraph_data:/var/lib/memgraph
      - ./memgraph.conf:/etc/memgraph/memgraph.conf
    restart: unless-stopped

  armchair:
    image: your-registry.com/armchair:latest
    ports:
      - "8080:8080"
    volumes:
      - ./production.yaml:/app/source.yaml:ro
      - ./logs:/app/logs
    environment:
      - GIN_MODE=release
      - NEO4J_URI=bolt://memgraph:7687
    depends_on:
      - memgraph
    restart: unless-stopped

volumes:
  memgraph_data:
```

## User Installation Instructions

### For End Users Running on Laptops

1. **Prerequisites**
   - Docker and Docker Compose installed
   - At least 2GB free disk space
   - Ports 8080 and 7687 available

2. **Download and Run**
   ```bash
   # Pull the latest image
   docker pull your-registry.com/armchair:latest
   
   # Create a basic configuration
   cat > source.yaml << EOF
   mcp:
     host: "localhost"
     port: 8282
   neo4j:
     uri: "bolt://memgraph:7687"
     username: ""
     password: ""
   source:
     repositories:
       - name: "my-project"
         path: "/workspace"
         language: "go"
   EOF
   
   # Run with Docker Compose
   docker-compose up -d
   ```

3. **Verify Installation**
   ```bash
   # Check service status
   docker-compose ps
   
   # Check logs
   docker-compose logs armchair
   
   # Test API endpoint
   curl http://localhost:8080/health
   ```

## Troubleshooting

### Common Issues

1. **Port Conflicts**
   - Change ports in docker-compose.yml if 8080 or 7687 are in use
   - Use `docker-compose ps` to see which ports are mapped

2. **Memgraph Connection Issues**
   - Ensure Memgraph is fully started before armchair connects
   - Check health status: `docker-compose exec memgraph mg_client -e "RETURN 1;"`

3. **Configuration Problems**
   - Verify source.yaml is properly mounted
   - Check file permissions on mounted volumes
   - View container logs: `docker-compose logs armchair`

4. **Build Issues**
   - Ensure Go 1.23+ is available in build environment
   - Check for network connectivity during `go mod download`
   - Verify all dependencies are accessible

### Debugging

```bash
# Access running container
docker-compose exec armchair sh

# View application logs
docker-compose logs -f armchair

# Check database connectivity
docker-compose exec armchair ./armchair -test
```

## Production Considerations

1. **Security**
   - Use non-root user in production containers
   - Set up proper firewall rules
   - Use TLS for external connections
   - Set strong database passwords

2. **Performance**
   - Allocate sufficient memory to Memgraph container
   - Use SSD storage for database volumes
   - Monitor container resource usage

3. **Monitoring**
   - Set up health checks for all services
   - Monitor logs for errors and performance issues
   - Use container orchestration (Kubernetes, Docker Swarm) for production

4. **Backup**
   - Regularly backup Memgraph data volume
   - Version your configuration files
   - Test disaster recovery procedures