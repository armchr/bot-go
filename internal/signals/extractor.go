package signals

import (
	"context"
	"fmt"
	"os"
	"strings"

	"bot-go/internal/model/ast"
	"bot-go/internal/service/codegraph"
	"bot-go/internal/service/ngram"
	"bot-go/internal/service/vector"
	"bot-go/internal/signals/model"
	"bot-go/internal/signals/utils"

	"go.uber.org/zap"
)

// ClassInfoExtractor builds ClassInfo from various sources
type ClassInfoExtractor struct {
	codeGraph    *codegraph.CodeGraph
	vectorDB     vector.VectorDatabase
	ngramService *ngram.NGramService
	logger       *zap.Logger
}

// NewClassInfoExtractor creates a new extractor
func NewClassInfoExtractor(
	codeGraph *codegraph.CodeGraph,
	vectorDB vector.VectorDatabase,
	ngramService *ngram.NGramService,
	logger *zap.Logger,
) *ClassInfoExtractor {
	return &ClassInfoExtractor{
		codeGraph:    codeGraph,
		vectorDB:     vectorDB,
		ngramService: ngramService,
		logger:       logger,
	}
}

// Extract builds a complete ClassInfo for a class
func (e *ClassInfoExtractor) Extract(ctx context.Context, repoName, className string) (*model.ClassInfo, error) {
	e.logger.Info("Extracting class info",
		zap.String("repo", repoName),
		zap.String("class", className))

	// 1. Find the class node in code graph
	classNode, err := e.findClassNode(ctx, repoName, className)
	if err != nil {
		return nil, fmt.Errorf("failed to find class node: %w", err)
	}

	// 2. Get file path from class node metadata
	filePath := e.codeGraph.GetFilePath(ctx, classNode.FileID)

	// 3. Read source code
	sourceCode, err := e.readSourceFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	// 4. Extract methods
	methods, err := e.extractMethods(ctx, classNode, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to extract methods: %w", err)
	}

	// 5. Extract fields
	fields, err := e.extractFields(ctx, classNode)
	if err != nil {
		return nil, fmt.Errorf("failed to extract fields: %w", err)
	}

	// 6. Detect accessors/mutators
	accessorDetector := utils.NewAccessorDetector()
	for _, method := range methods {
		method.IsAccessor = accessorDetector.IsAccessor(method.Name, method.SourceCode)
	}

	classInfo := &model.ClassInfo{
		RepoName:     repoName,
		ClassName:    className,
		FilePath:     filePath,
		FileID:       classNode.FileID,
		ClassNode:    classNode,
		Methods:      methods,
		Fields:       fields,
		SourceCode:   sourceCode,
		StartLine:    classNode.Range.Start.Line,
		EndLine:      classNode.Range.End.Line,
		CodeGraph:    e.codeGraph,
		VectorDB:     e.vectorDB,
		NGramService: e.ngramService,
	}

	e.logger.Info("Extracted class info",
		zap.String("class", className),
		zap.Int("methods", len(methods)),
		zap.Int("fields", len(fields)),
		zap.Int("loc", classInfo.GetLOC()))

	return classInfo, nil
}

// ExtractAll extracts all classes in a repository
func (e *ClassInfoExtractor) ExtractAll(ctx context.Context, repoName string) ([]*model.ClassInfo, error) {
	e.logger.Info("Extracting all classes in repository",
		zap.String("repo", repoName))

	// Query all class nodes in the repository
	classNodes, err := e.findAllClasses(ctx, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to find classes: %w", err)
	}

	var result []*model.ClassInfo
	for _, classNode := range classNodes {
		className := classNode.Name
		classInfo, err := e.Extract(ctx, repoName, className)
		if err != nil {
			e.logger.Warn("Failed to extract class info",
				zap.String("class", className),
				zap.Error(err))
			continue
		}
		result = append(result, classInfo)
	}

	e.logger.Info("Extracted all classes",
		zap.String("repo", repoName),
		zap.Int("count", len(result)))

	return result, nil
}

// findClassNode queries the code graph for a class node
func (e *ClassInfoExtractor) findClassNode(ctx context.Context, repoName, className string) (*ast.Node, error) {
	// Query code graph for class node
	// This is a simplified version - actual implementation would query Neo4j/Kuzu
	nodes, err := e.codeGraph.GetNodesByName(ctx, className, ast.NodeTypeClass)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("class not found: %s", className)
	}

	// If multiple matches, pick the first one (could be refined with file path filter)
	return nodes[0], nil
}

// findAllClasses queries all class nodes in a repository
func (e *ClassInfoExtractor) findAllClasses(ctx context.Context, repoName string) ([]*ast.Node, error) {
	// Query all class nodes from code graph
	// This would be a Cypher query like: MATCH (c:Class) WHERE c.repo = $repoName RETURN c
	nodes, err := e.codeGraph.GetNodesByType(ctx, ast.NodeTypeClass)
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

// extractMethods gets all methods in a class
func (e *ClassInfoExtractor) extractMethods(ctx context.Context, classNode *ast.Node, sourceCode []byte) ([]*model.MethodInfo, error) {
	// Query methods via CONTAINS relationship
	methodNodes, err := e.codeGraph.GetChildNodes(ctx, classNode.ID, "CONTAINS", ast.NodeTypeFunction)
	if err != nil {
		return nil, err
	}

	var methods []*model.MethodInfo
	sourceLines := strings.Split(string(sourceCode), "\n")

	for _, methodNode := range methodNodes {
		// Extract method source code
		startLine := methodNode.Range.Start.Line
		endLine := methodNode.Range.End.Line

		// Bounds check
		if startLine < 1 || endLine > len(sourceLines) {
			e.logger.Warn("Method line range out of bounds",
				zap.String("method", methodNode.Name),
				zap.Int("start", startLine),
				zap.Int("end", endLine))
			continue
		}

		// Extract lines (convert to 0-indexed)
		//methodSource := strings.Join(sourceLines[startLine-1:endLine], "\n")

		method := &model.MethodInfo{
			Node:       methodNode,
			Name:       methodNode.Name,
			//SourceCode: []byte(methodSource),
			StartLine:  startLine,
			EndLine:    endLine,
			Complexity: -1, // Not computed yet
			Entropy:    -1, // Not computed yet
		}

		methods = append(methods, method)
	}

	return methods, nil
}

// extractFields gets all fields in a class
func (e *ClassInfoExtractor) extractFields(ctx context.Context, classNode *ast.Node) ([]*ast.Node, error) {
	// Query fields via HAS_FIELD relationship
	fields, err := e.codeGraph.GetChildNodes(ctx, classNode.ID, "HAS_FIELD", ast.NodeTypeField)
	if err != nil {
		return nil, err
	}

	return fields, nil
}

// readSourceFile reads the source code file
func (e *ClassInfoExtractor) readSourceFile(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return content, nil
}
