package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"bot-go/internal/config"
	"bot-go/internal/model"
	"bot-go/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

type CodeGraphServer struct {
	server      *mcp.Server
	repoService *service.RepoService
	config      *config.Config
	logger      *zap.Logger
	handler     *mcp.StreamableHTTPHandler
}

type CallGraphParams struct {
	RepoName     string `json:"repo_name" jsonschema:"the name of the repository to analyze"`
	FunctionName string `json:"function_name,omitempty" jsonschema:"specific function to analyze"`
	FilePath     string `json:"file_path,omitempty" jsonschema:"specific file path containing the function"`
}

func NewCodeGraphServer(repoService *service.RepoService, cfg *config.Config, logger *zap.Logger) *CodeGraphServer {
	server := &CodeGraphServer{
		repoService: repoService,
		config:      cfg,
		logger:      logger,
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "CodeInsight",
		Version: "1.0.0",
	}, nil)

	// Register the getCallGraph tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "getCallGraph",
		Description: "Retrieve the call graph for a given function in a file. Returns a graph with each function being called, their location and their call graph",
	}, server.handleCallGraph)

	// Register the getCallerGraph tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "getCallerGraph",
		Description: "Retrieve the caller graph for a given function in a file. Returns a graph with each function calling this function, their location and their caller graph",
	}, server.handleCallerGraph)

	server.handler = mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	server.server = mcpServer
	return server
}

func (s *CodeGraphServer) handleCallGraph(ctx context.Context, req *mcp.CallToolRequest, args CallGraphParams) (*mcp.CallToolResult, any, error) {
	s.logger.Info("Handling callGraph request", zap.String("repo_name", args.RepoName), zap.String("function_name", args.FunctionName))

	// Get repository configuration
	repo, err := s.config.GetRepository(args.RepoName)
	if err != nil {
		s.logger.Error("Repository not found", zap.String("repo_name", args.RepoName), zap.Error(err))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Repository not found: %s", args.RepoName)}},
		}, nil, nil
	}

	// Generate call graph analysis
	callGraph, err := s.generateCallGraph(ctx, repo, args.FilePath, args.FunctionName)
	if err != nil {
		s.logger.Error("Failed to generate call graph", zap.String("repo_name", args.RepoName), zap.Error(err))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to generate call graph: %v", err)}},
		}, nil, nil
	}

	//result := fmt.Sprintf("Call graph analysis for repository '%s':\n%v", args.RepoName, callGraph)
	result := s.formatCallGraph(ctx, args.RepoName, callGraph)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil, nil
}

func (s *CodeGraphServer) generateCallGraph(ctx context.Context, repo *config.Repository, filePath string, targetFunction string) (*model.CallGraph, error) {
	// Initialize LSP client to get more detailed analysis
	callGraph, err := s.repoService.GetFunctionDependencies(ctx, repo.Name, filePath, targetFunction, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to get function dependencies: %w", err)
	}
	return callGraph, nil
}

func (s *CodeGraphServer) handleCallerGraph(ctx context.Context, req *mcp.CallToolRequest, args CallGraphParams) (*mcp.CallToolResult, any, error) {
	s.logger.Info("Handling callerGraph request", zap.String("repo_name", args.RepoName), zap.String("function_name", args.FunctionName))

	// Get repository configuration
	repo, err := s.config.GetRepository(args.RepoName)
	if err != nil {
		s.logger.Error("Repository not found", zap.String("repo_name", args.RepoName), zap.Error(err))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Repository not found: %s", args.RepoName)}},
		}, nil, nil
	}

	// Generate caller graph analysis
	callerGraph, err := s.generateCallerGraph(ctx, repo, args.FilePath, args.FunctionName)
	if err != nil {
		s.logger.Error("Failed to generate caller graph", zap.String("repo_name", args.RepoName), zap.Error(err))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to generate caller graph: %v", err)}},
		}, nil, nil
	}

	result := s.formatCallerGraph(ctx, args.RepoName, callerGraph)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil, nil
}

func (s *CodeGraphServer) generateCallerGraph(ctx context.Context, repo *config.Repository, filePath string, targetFunction string) (*model.CallGraph, error) {
	// Initialize LSP client to get caller analysis
	callerGraph, err := s.repoService.GetFunctionCallers(ctx, repo.Name, filePath, targetFunction, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to get function callers: %w", err)
	}
	return callerGraph, nil
}

func (s *CodeGraphServer) formatCallGraph(ctx context.Context, repoName string, cg *model.CallGraph) string {
	if cg == nil {
		return "No call graph available."
	}

	if len(cg.Roots) == 0 {
		return "No root functions found in call graph."
	}

	// Collect all unique functions from the call graph
	allFunctions := make([]model.FunctionDefinition, 0)
	functionMap := make(map[string]bool)
	
	// Add root functions
	for _, root := range cg.Roots {
		key := root.ToKey()
		if !functionMap[key] {
			allFunctions = append(allFunctions, root)
			functionMap[key] = true
		}
	}
	
	// Add all other functions from edges
	for _, fn := range cg.Functions {
		key := fn.ToKey()
		if !functionMap[key] {
			allFunctions = append(allFunctions, fn)
			functionMap[key] = true
		}
	}

	// Get hover information for all functions
	hoverStrings, err := s.repoService.GetFunctionHovers(ctx, repoName, allFunctions)
	if err != nil {
		s.logger.Warn("Failed to get hover information for functions", zap.Error(err))
		// Create empty hover strings as fallback
		hoverStrings = make([]string, len(allFunctions))
	}
	
	// Create hover lookup map
	hoverMap := make(map[string]string)
	for i, fn := range allFunctions {
		hoverMap[fn.ToKey()] = hoverStrings[i]
	}

	// Build adjacency map for efficient edge traversal
	adjacencyMap := make(map[string][]*model.FunctionDefinition)
	for _, edge := range cg.Edges {
		if edge.From != nil {
			fromKey := edge.From.ToKey()
			adjacencyMap[fromKey] = append(adjacencyMap[fromKey], edge.To)
		}
	}

	var result strings.Builder

	// Process each root function
	for i, root := range cg.Roots {
		if i > 0 {
			result.WriteString("\n\n")
		}
		visited := make(map[string]bool)
		s.formatCallGraphNode(&root, adjacencyMap, hoverMap, visited, 0, &result)
	}

	return result.String()
}

func (s *CodeGraphServer) formatCallGraphNode(node *model.FunctionDefinition, adjacencyMap map[string][]*model.FunctionDefinition, hoverMap map[string]string, visited map[string]bool, depth int, result *strings.Builder) {
	if node == nil {
		return
	}

	// Create indentation
	indent := strings.Repeat("    ", depth)

	// Extract file path from URI (remove file:// prefix if present)
	filePath := node.Location.URI
	if strings.HasPrefix(filePath, "file://") {
		filePath = filePath[7:]
	}

	// Get hover information for this node
	nodeKey := node.ToKey()
	hoverInfo := hoverMap[nodeKey]

	// Write the function node with hover information
	if hoverInfo != "" {
		// Clean up hover info for better display
		hoverInfo = strings.ReplaceAll(hoverInfo, "\n", " ")
		if len(hoverInfo) > 200 {
			hoverInfo = hoverInfo[:200] + "..."
		}
		result.WriteString(fmt.Sprintf("%s<step> %s (file: %s)\n%s  Description: %s\n", indent, node.Name, filePath, indent, hoverInfo))
	} else {
		result.WriteString(fmt.Sprintf("%s<step> %s (file: %s)\n", indent, node.Name, filePath))
	}

	// Get children from adjacency map
	if children, exists := adjacencyMap[nodeKey]; exists && !visited[nodeKey] {
		visited[nodeKey] = true

		// Process each child
		for _, child := range children {
			s.formatCallGraphNode(child, adjacencyMap, hoverMap, visited, depth+1, result)
		}

		visited[nodeKey] = false // Allow revisiting in different branches
	}

	// Close the step tag
	result.WriteString(fmt.Sprintf("%s</step>\n", indent))
}

func (s *CodeGraphServer) formatCallerGraph(ctx context.Context, repoName string, cg *model.CallGraph) string {
	if cg == nil {
		return "No caller graph available."
	}

	if len(cg.Roots) == 0 {
		return "No root functions found in caller graph."
	}

	// Collect all unique functions from the call graph
	allFunctions := make([]model.FunctionDefinition, 0)
	functionMap := make(map[string]bool)
	
	// Add root functions
	for _, root := range cg.Roots {
		key := root.ToKey()
		if !functionMap[key] {
			allFunctions = append(allFunctions, root)
			functionMap[key] = true
		}
	}
	
	// Add all other functions from edges
	for _, fn := range cg.Functions {
		key := fn.ToKey()
		if !functionMap[key] {
			allFunctions = append(allFunctions, fn)
			functionMap[key] = true
		}
	}

	// Get hover information for all functions
	hoverStrings, err := s.repoService.GetFunctionHovers(ctx, repoName, allFunctions)
	if err != nil {
		s.logger.Warn("Failed to get hover information for functions", zap.Error(err))
		// Create empty hover strings as fallback
		hoverStrings = make([]string, len(allFunctions))
	}
	
	// Create hover lookup map
	hoverMap := make(map[string]string)
	for i, fn := range allFunctions {
		hoverMap[fn.ToKey()] = hoverStrings[i]
	}

	// Build adjacency map for efficient edge traversal
	adjacencyMap := make(map[string][]*model.FunctionDefinition)
	for _, edge := range cg.Edges {
		if edge.From != nil {
			fromKey := edge.From.ToKey()
			adjacencyMap[fromKey] = append(adjacencyMap[fromKey], edge.To)
		}
	}

	var result strings.Builder

	// Process each root function
	for i, root := range cg.Roots {
		if i > 0 {
			result.WriteString("\n\n")
		}
		visited := make(map[string]bool)
		s.formatCallerGraphNode(&root, adjacencyMap, hoverMap, visited, 0, &result)
	}

	return result.String()
}

func (s *CodeGraphServer) formatCallerGraphNode(node *model.FunctionDefinition, adjacencyMap map[string][]*model.FunctionDefinition, hoverMap map[string]string, visited map[string]bool, depth int, result *strings.Builder) {
	if node == nil {
		return
	}

	// Create indentation
	indent := strings.Repeat("    ", depth)

	// Extract file path from URI (remove file:// prefix if present)
	filePath := node.Location.URI
	if strings.HasPrefix(filePath, "file://") {
		filePath = filePath[7:]
	}

	// Get hover information for this node
	nodeKey := node.ToKey()
	hoverInfo := hoverMap[nodeKey]

	// Write the function node with hover information using caller tags
	if hoverInfo != "" {
		// Clean up hover info for better display
		hoverInfo = strings.ReplaceAll(hoverInfo, "\n", " ")
		if len(hoverInfo) > 200 {
			hoverInfo = hoverInfo[:200] + "..."
		}
		result.WriteString(fmt.Sprintf("%s<caller> %s (file: %s)\n%s  Description: %s\n", indent, node.Name, filePath, indent, hoverInfo))
	} else {
		result.WriteString(fmt.Sprintf("%s<caller> %s (file: %s)\n", indent, node.Name, filePath))
	}

	// Get children from adjacency map
	if children, exists := adjacencyMap[nodeKey]; exists && !visited[nodeKey] {
		visited[nodeKey] = true

		// Process each child
		for _, child := range children {
			s.formatCallerGraphNode(child, adjacencyMap, hoverMap, visited, depth+1, result)
		}

		visited[nodeKey] = false // Allow revisiting in different branches
	}

	// Close the caller tag
	result.WriteString(fmt.Sprintf("%s</caller>\n", indent))
}

/*
func (s *CodeGraphServer) handleCallGraphHTTP(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	// Convert HTTP arguments to CallGraphParams
	var params CallGraphParams
	if repoName, ok := arguments["repo_name"].(string); ok {
		params.RepoName = repoName
	} else {
		return nil, fmt.Errorf("repo_name is required and must be a string")
	}

	if functionName, ok := arguments["function_name"].(string); ok {
		params.FunctionName = functionName
	}

	// Call the MCP handler and extract the result
	mcpResult, _, err := s.handleCallGraph(ctx, nil, params)
	if err != nil {
		return nil, err
	}

	// Extract text content from MCP result
	if len(mcpResult.Content) > 0 {
		if textContent, ok := mcpResult.Content[0].(*mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "Call graph analysis completed", nil
}
*/

func (s *CodeGraphServer) SetupHTTPRoutes(router *gin.Engine) {
	go func() {
		address := s.config.Mcp.GetAddress()
		log.Printf("MCP Server going to listen on %s", address)
		if err := http.ListenAndServe(address, s.handler); err != nil {
			log.Fatalf("MCP Server failed: %v", err)
		}
	}()

	/*
		mcpGroup := router.Group("/mcp")
		{
			// HTTP transport endpoints for MCP
			mcpGroup.GET("/messages", s.handleSSEConnection)
			mcpGroup.POST("/messages", s.handleHTTPRequest)
			mcpGroup.GET("/health", s.handleHealthCheck)
		}
	*/
}

/*
func (s *CodeGraphServer) handleSSEConnection(c *gin.Context) {
	// Server-Sent Events endpoint for bidirectional communication
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Keep connection alive for MCP protocol
	c.String(http.StatusOK, "data: {\"type\":\"connection_established\"}\n\n")
	c.Writer.Flush()
}


func (s *CodeGraphServer) handleHTTPRequest(c *gin.Context) {
	var request map[string]interface{}
	if err := c.ShouldBindJSON(&request); err != nil {
		s.logger.Error("Invalid JSON request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	s.logger.Info("Received MCP HTTP request", zap.Any("request", request))

	// Handle different MCP message types
	method, ok := request["method"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid method"})
		return
	}

	switch method {
	case "tools/list":
		s.handleToolsList(c)
	case "tools/call":
		s.handleToolsCall(c, request)
	case "initialize":
		s.handleInitialize(c, request)
	default:
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Method not implemented: " + method})
	}
}

func (s *CodeGraphServer) handleInitialize(c *gin.Context, request map[string]interface{}) {
	response := map[string]interface{}{
		"id": request["id"],
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "CodeGraph",
				"version": "1.0.0",
			},
		},
	}
	c.JSON(http.StatusOK, response)
}

func (s *CodeGraphServer) handleToolsList(c *gin.Context) {
	tools := []map[string]interface{}{
		{
			"name":        "callGraph",
			"description": "Analyzes a repository to generate a call graph of functions and their relationships",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo_name": map[string]interface{}{
						"type":        "string",
						"description": "The name of the repository to analyze",
					},
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to analyze in the repo",
					},
					"function_name": map[string]interface{}{
						"type":        "string",
						"description": "Optional: specific function to analyze",
					},
				},
				"required": []string{"repo_name", "function_name"},
			},
		},
	}

	response := map[string]interface{}{
		"result": map[string]interface{}{
			"tools": tools,
		},
	}
	c.JSON(http.StatusOK, response)
}

func (s *CodeGraphServer) handleToolsCall(c *gin.Context, request map[string]interface{}) {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid params"})
		return
	}

	toolName, ok := params["name"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid tool name"})
		return
	}

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		arguments = make(map[string]interface{})
	}

	ctx := c.Request.Context()
	var result interface{}
	var err error

	switch strings.ToLower(toolName) {
	case "callgraph":
		result, err = s.handleCallGraphHTTP(ctx, arguments)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown tool: " + toolName})
		return
	}

	if err != nil {
		s.logger.Error("Tool execution failed", zap.String("tool", toolName), zap.Error(err))
		response := map[string]interface{}{
			"id": request["id"],
			"error": map[string]interface{}{
				"code":    -1,
				"message": err.Error(),
			},
		}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	response := map[string]interface{}{
		"id": request["id"],
		"result": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Call graph analysis completed successfully: %v", result),
				},
			},
		},
	}
	c.JSON(http.StatusOK, response)
}

func (s *CodeGraphServer) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "CodeGraph MCP Server",
		"version": "1.0.0",
	})
}
*/
