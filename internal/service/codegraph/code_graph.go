package codegraph

import (
	"context"
	"fmt"
	"strings"

	"bot-go/internal/config"
	"bot-go/internal/model/ast"
	"bot-go/pkg/lsp/base"

	"go.uber.org/zap"
)

type CodeGraph struct {
	db          GraphDatabase
	config      *config.Config
	logger      *zap.Logger
	fileIDCache map[int32]string
}

func NewCodeGraph(uri, username, password string, config *config.Config, logger *zap.Logger) (*CodeGraph, error) {
	db, err := NewNeo4jDatabase(uri, username, password, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j database: %w", err)
	}

	err = db.VerifyConnectivity(context.Background())
	if err != nil {
		db.Close(context.Background())
		return nil, fmt.Errorf("failed to verify database connectivity: %w", err)
	}

	return &CodeGraph{
		db:          db,
		config:      config,
		logger:      logger,
		fileIDCache: make(map[int32]string),
	}, nil
}

// NewCodeGraphWithKuzu creates a new CodeGraph instance using Kuzu database
func NewCodeGraphWithKuzu(config *config.Config, logger *zap.Logger) (*CodeGraph, error) {
	// Use the database path from config, fallback to in-memory if not specified
	databasePath := config.Kuzu.Path
	if databasePath == "" {
		databasePath = ":memory:"
		logger.Info("No Kuzu database path configured, using in-memory database")
	}

	db, err := NewKuzuDatabase(databasePath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kuzu database: %w", err)
	}

	err = db.VerifyConnectivity(context.Background())
	if err != nil {
		db.Close(context.Background())
		return nil, fmt.Errorf("failed to verify database connectivity: %w", err)
	}

	return &CodeGraph{
		db:          db,
		config:      config,
		logger:      logger,
		fileIDCache: make(map[int32]string),
	}, nil
}

func (cg *CodeGraph) Close(ctx context.Context) error {
	return cg.db.Close(ctx)
}

func (cg *CodeGraph) dbRecordToNode(record GraphNode) (*ast.Node, error) {
	recordMap := make(map[string]any)
	for key, value := range record.GetProperties() {
		recordMap[key] = value
	}

	return cg.recordToNode(recordMap)
}

func (cg *CodeGraph) recordToNode(record map[string]any) (*ast.Node, error) {
	id := record["id"]
	nodeType := record["nodeType"]
	fileID := record["fileId"]
	name := record["name"]
	rangeStr := record["range"]
	version := record["version"]
	scopeID := record["scopeId"]

	newMetadata := make(map[string]any)
	for key, value := range record {
		if cg.isFirstClassMetadata(key) {
			newMetadata[key] = value
		} else if strings.HasPrefix(key, "md_") {
			newMetadata[key[3:]] = value
		}
	}

	node := &ast.Node{
		ID:       ast.NodeID(cg.convertToInt64(id)),
		NodeType: ast.NodeType(cg.convertToInt64(nodeType)),
		FileID:   cg.convertToInt32(fileID),
		Name:     name.(string),
		Version:  cg.convertToInt32(version),
		ScopeID:  ast.NodeID(cg.convertToInt64(scopeID)),
	}

	if rangeStr != nil {
		node.Range = strToRange(rangeStr.(string))
	}

	if len(newMetadata) > 0 {
		node.MetaData = newMetadata
	}

	return node, nil
}

/*
func parseRange(rangeMap map[string]any) base.Range {
	var rng base.Range

	if startMap, ok := rangeMap["start"].(map[string]any); ok {
		if line, ok := startMap["line"].(int64); ok {
			rng.Start.Line = int(line)
		}
		if char, ok := startMap["character"].(int64); ok {
			rng.Start.Character = int(char)
		}
	}

	if endMap, ok := rangeMap["end"].(map[string]any); ok {
		if line, ok := endMap["line"].(int64); ok {
			rng.End.Line = int(line)
		}
		if char, ok := endMap["character"].(int64); ok {
			rng.End.Character = int(char)
		}
	}

	return rng
}
*/

func (cg *CodeGraph) getNodeLabel(nodeType ast.NodeType) string {
	switch nodeType {
	case ast.NodeTypeModuleScope:
		return "ModuleScope"
	case ast.NodeTypeFileScope:
		return "FileScope"
	case ast.NodeTypeBlock:
		return "Block"
	case ast.NodeTypeVariable:
		return "Variable"
	case ast.NodeTypeExpression:
		return "Expression"
	case ast.NodeTypeConditional:
		return "Conditional"
	case ast.NodeTypeFunction:
		return "Function"
	case ast.NodeTypeClass:
		return "Class"
	case ast.NodeTypeField:
		return "Field"
	case ast.NodeTypeFunctionCall:
		return "FunctionCall"
	case ast.NodeTypeFileNumber:
		return "FileNumber"
	case ast.NodeTypeLoop:
		return "Loop"
	default:
		return "Node"
	}
}

func (cg *CodeGraph) CreateFunction(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeFunction {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeFunction, node.NodeType)
	}
	err := cg.writeNode(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to create function node: %w", err)
	}

	return nil
}

func (cg *CodeGraph) ReadFunction(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeFunction)
}

/*
func (cg *CodeGraph) ReadFunctionArgs(ctx context.Context, functionNodeID ast.NodeID) ([]*ast.Node, error) {
	session := cg.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	nodesResult, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (f:Function {id: $functionId})-[:FUNCTION_ARG]->(arg)
			RETURN arg
			ORDER BY arg.position
		`
		parameters := map[string]any{
			"functionId": int64(functionNodeID),
		}

		result, err := tx.Run(ctx, query, parameters)
		if err != nil {
			return nil, err
		}

		var nodes []*ast.Node
		for result.Next(ctx) {
			record := result.Record()
			node, err := cg.recordToNode(record)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}

		if err = result.Err(); err != nil {
			return nil, err
		}

		return nodes, nil
	})

	if err != nil {
		cg.logger.Error("Failed to read function arguments", zap.Int64("functionId", int64(functionNodeID)), zap.Error(err))
		return nil, fmt.Errorf("failed to read function arguments: %w", err)
	}

	return nodesResult.([]*ast.Node), nil
}
*/

func (cg *CodeGraph) CreateFileScope(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeFileScope {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeFileScope, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadFileScope(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeFileScope)
}

func (cg *CodeGraph) GetFilePath(ctx context.Context, fileID int32) string {
	if path, ok := cg.fileIDCache[fileID]; ok {
		return path
	}

	fs, err := cg.ReadFileScope(ctx, ast.NodeID(fileID))
	if err != nil {
		return ""
	}
	path, ok := fs.MetaData["path"].(string)
	if !ok {
		return ""
	}
	cg.fileIDCache[fileID] = path
	return path
}

func (cg *CodeGraph) FindFileScopes(ctx context.Context, repoName, filePath string) ([]*ast.Node, error) {
	params := map[string]any{
		"repo": repoName,
	}

	if filePath != "" {
		params["path"] = filePath
	}
	nodes, err := cg.readNodes(ctx, ast.NodeTypeFileScope, params)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (cg *CodeGraph) CreateClass(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeClass {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeClass, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadClass(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeClass)
}

func (cg *CodeGraph) CreateVariable(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeVariable {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeVariable, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadVariable(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeVariable)
}

func (cg *CodeGraph) CreateBlock(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeBlock {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeBlock, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadBlock(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeBlock)
}

func (cg *CodeGraph) CreateExpression(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeExpression {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeExpression, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadExpression(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeExpression)
}

func (cg *CodeGraph) CreateConditional(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeConditional {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeConditional, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadConditional(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeConditional)
}

func (cg *CodeGraph) CreateLoop(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeLoop {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeLoop, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) CreateField(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeField {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeField, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadField(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeField)
}

func (cg *CodeGraph) CreateFunctionCall(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeFunctionCall {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeFunctionCall, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadFunctionCall(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeFunctionCall)
}

func (cg *CodeGraph) CreateModuleScope(ctx context.Context, node *ast.Node) error {
	if node.NodeType != ast.NodeTypeModuleScope {
		return fmt.Errorf("invalid node type: expected %d, got %d", ast.NodeTypeModuleScope, node.NodeType)
	}
	return cg.writeNode(ctx, node)
}

func (cg *CodeGraph) ReadModuleScope(ctx context.Context, nodeID ast.NodeID) (*ast.Node, error) {
	return cg.readNodeByType(ctx, nodeID, ast.NodeTypeModuleScope)
}

func rangeToString(rng base.Range) string {
	return fmt.Sprintf("(%d,%d)-(%d,%d)", rng.Start.Line, rng.Start.Character, rng.End.Line, rng.End.Character)
}

func strToRange(s string) base.Range {
	var rng base.Range
	_, err := fmt.Sscanf(s, "(%d,%d)-(%d,%d)", &rng.Start.Line, &rng.Start.Character, &rng.End.Line, &rng.End.Character)
	if err != nil {
		return base.Range{}
	}
	return rng
}

var (
	FirstClassMetadata = map[string]bool{
		"fake":   true,
		"nameID": true,
		"return": true,
		"repo":   true,
		"path":   true,
	}
)

func (cg *CodeGraph) isFirstClassMetadata(key string) bool {
	return FirstClassMetadata[key]
}

func (cg *CodeGraph) populateFirstClassMetadata(metadata map[string]any,
	param map[string]any,
	newMetadata map[string]any) {
	for key, value := range metadata {
		if cg.isFirstClassMetadata(key) {
			param[key] = value
		} else {
			newMetadata[key] = value
		}
	}
}

func (cg *CodeGraph) mapToSetParamString(m map[string]any, varName string) string {
	if len(m) == 0 {
		return ""
	}

	setClauses := ""
	for key := range m {
		if setClauses != "" {
			setClauses += ",\n"
		}
		setClauses += fmt.Sprintf("%s.%s = $%s", varName, key, key)
	}
	return setClauses
}

func (cg *CodeGraph) flattenMetadata(metadata map[string]any, param map[string]any) {
	for key, value := range metadata {
		param["md_"+key] = value
	}
}

func (cg *CodeGraph) writeNode(ctx context.Context, node *ast.Node) error {
	nodeLabel := cg.getNodeLabel(node.NodeType)
	parameters := map[string]any{
		"id":       int64(node.ID),
		"nodeType": int64(node.NodeType),
		"fileId":   int64(node.FileID),
		"name":     node.Name,
		"range":    rangeToString(node.Range),
		"version":  int64(node.Version),
		"scopeId":  int64(node.ScopeID),
	}

	if node.MetaData != nil {
		newMetadata := make(map[string]any)
		cg.populateFirstClassMetadata(node.MetaData, parameters, newMetadata)
		if len(newMetadata) > 0 {
			cg.flattenMetadata(newMetadata, parameters)
			//parameters["metaData"] = newMetadata
		}
	}

	cg.logger.Debug("Writing node", zap.Int64("nodeId", int64(node.ID)), zap.Any("parameters", parameters))

	setQ := cg.mapToSetParamString(parameters, "n")
	query := fmt.Sprintf(`
		MERGE (n:%s {id: $id})
		SET %s
		RETURN n
	`, nodeLabel, setQ)

	_, err := cg.db.ExecuteWrite(ctx, query, parameters)
	if err != nil {
		cg.logger.Error("Failed to write node", zap.Int64("nodeId", int64(node.ID)), zap.Error(err))
		return fmt.Errorf("failed to write node: %w", err)
	}

	return nil
}

func (cg *CodeGraph) readNodes(ctx context.Context, nodeType ast.NodeType, query map[string]any) ([]*ast.Node, error) {
	nodeLabel := cg.getNodeLabel(nodeType)
	q := ""
	if len(query) > 0 {
		q = "WHERE "
		i := 0
		for key := range query {
			if i > 0 {
				q += " AND\n"
			}
			q += fmt.Sprintf("n.%s = $%s", key, key)
			i++
		}
	}

	// For Kuzu, we need to handle the query differently since it doesn't use labels in MATCH the same way
	fullQuery := fmt.Sprintf(`
		MATCH (n:%s)
		%s
		RETURN n
	`, nodeLabel, q)

	records, err := cg.db.ExecuteRead(ctx, fullQuery, query)
	if err != nil {
		cg.logger.Error("Failed to read node",
			zap.Int64("nodeType", int64(nodeType)),
			zap.Error(err))
		return nil, fmt.Errorf("failed to read node: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("node query with type %d not found", nodeType)
	}

	var results []*ast.Node
	for _, record := range records {
		nodeData, ok := record["n"]
		if !ok || nodeData == nil {
			continue
		}

		// Convert map to our GraphNode interface and then to ast.Node
		nodeMap, ok := nodeData.(map[string]any)
		if !ok {
			continue
		}

		node, err := cg.recordToNode(nodeMap)
		if err != nil {
			return nil, err
		}

		results = append(results, node)
	}

	return results, nil
}

func (cg *CodeGraph) readNodeByType(ctx context.Context, nodeID ast.NodeID, nodeType ast.NodeType) (*ast.Node, error) {
	nodes, err := cg.readNodes(ctx, nodeType, map[string]any{"id": int64(nodeID)})
	if err != nil {
		return nil, err
	}
	if len(nodes) != 1 {
		return nil, fmt.Errorf("node with id %d and type %d found - expected 1 but got %d", nodeID, nodeType, len(nodes))
	}
	return nodes[0], nil
}

func (cg *CodeGraph) CreateRelation(ctx context.Context, parentNodeID, childNodeID ast.NodeID,
	relationLabel string, metaData map[string]any) error {
	parameters := map[string]any{
		"parentId": int64(parentNodeID),
		"childId":  int64(childNodeID),
	}

	setMetaDataQ := ""
	if metaData != nil {
		//parameters["metaData"] = metaData
		//setMetaDataQ = "SET r.metaData = $metaData"
		newMetadata := make(map[string]any)
		cg.flattenMetadata(metaData, newMetadata)
		setMetaDataQ = cg.mapToSetParamString(newMetadata, "r")
		if setMetaDataQ != "" {
			setMetaDataQ = "SET " + setMetaDataQ
		}
		// append newMetadata to parameters
		for key, value := range newMetadata {
			parameters[key] = value
		}
	}

	query := fmt.Sprintf(`
		MATCH (parent {id: $parentId}), (child {id: $childId})
		MERGE (parent)-[r:%s]->(child)
		%s
		RETURN parent, child
	`, relationLabel, setMetaDataQ)

	_, err := cg.db.ExecuteWrite(ctx, query, parameters)
	if err != nil {
		cg.logger.Error("Failed to create relation",
			zap.Int64("parentId", int64(parentNodeID)),
			zap.Int64("childId", int64(childNodeID)),
			zap.String("relationLabel", relationLabel),
			zap.Error(err))
		return fmt.Errorf("failed to create relation: %w", err)
	}

	return nil
}

func (cg *CodeGraph) CreateContainsRelation(ctx context.Context, parentNodeID, childNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, parentNodeID, childNodeID, "CONTAINS", nil)
}

func (cg *CodeGraph) CreateHasFieldRelation(ctx context.Context, parentNodeID, childNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, parentNodeID, childNodeID, "HAS_FIELD", nil)
}
func (cg *CodeGraph) CreateCallsRelation(ctx context.Context, callerNodeID, calleeNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, callerNodeID, calleeNodeID, "CALLS", nil)
}

/*
func (cg *CodeGraph) CreateContainedByRelation(ctx context.Context, parentNodeID, childNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, parentNodeID, childNodeID, "CONTAINED_BY", nil)
}
*/

func (cg *CodeGraph) CreateInheritsRelation(ctx context.Context, parentNodeID, childNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, parentNodeID, childNodeID, "INHERITS", nil)
}

func (cg *CodeGraph) CreateCallsFunctionRelation(ctx context.Context, callerNodeID, calleeNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, callerNodeID, calleeNodeID, "CALLS_FUNCTION", nil)
}

func (cg *CodeGraph) CreateUsesVariableRelation(ctx context.Context, userNodeID, variableNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, userNodeID, variableNodeID, "USES_VARIABLE", nil)
}

func (cg *CodeGraph) CreateImportsRelation(ctx context.Context, importerNodeID, importedNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, importerNodeID, importedNodeID, "IMPORTS", nil)
}

func (cg *CodeGraph) CreateBodyRelation(ctx context.Context, parentNodeID, bodyNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, parentNodeID, bodyNodeID, "BODY", nil)
}

func (cg *CodeGraph) CreateAnnotationRelation(ctx context.Context, parentNodeID, annotationNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, parentNodeID, annotationNodeID, "ANNOTATION", nil)
}

func (cg *CodeGraph) CreateFunctionArgRelation(ctx context.Context, functionNodeID, argNodeID ast.NodeID,
	position int) error {
	return cg.CreateRelation(ctx, functionNodeID, argNodeID, "FUNCTION_ARG", map[string]any{
		"position": position,
	})
}

func (cg *CodeGraph) CreateFromRelation(ctx context.Context, fromNodeID, toNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, fromNodeID, toNodeID, "FROM", nil)
}

func (cg *CodeGraph) CreateDataFlowRelation(ctx context.Context, sourceNodeID, targetNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, sourceNodeID, targetNodeID, "DATA_FLOW", nil)
}

func (cg *CodeGraph) CreateFunctionCallArgRelation(ctx context.Context, callNodeID, argNodeID ast.NodeID,
	position int) error {
	return cg.CreateRelation(ctx, callNodeID, argNodeID, "FUNCTION_CALL_ARG", map[string]any{
		"position": position,
	})
}

func (cg *CodeGraph) CreateReturnsRelation(ctx context.Context, functionNodeID, returnNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, functionNodeID, returnNodeID, "RETURNS", nil)
}

func (cg *CodeGraph) CreateAliasRelation(ctx context.Context, aliasNodeID, originalNodeID ast.NodeID) error {
	return cg.CreateRelation(ctx, aliasNodeID, originalNodeID, "ALIAS", nil)
}

func (cg *CodeGraph) CreateConditionalRelation(ctx context.Context, condNodeID,
	branchNodeID ast.NodeID, position int, conditionID ast.NodeID) error {
	return cg.CreateRelation(ctx, condNodeID, branchNodeID, "BRANCH", map[string]any{
		"position":  position,
		"condition": conditionID,
	})
}

/*func (cg *CodeGraph) GetOrCreateNextFileID(ctx context.Context) (int32, error) {
	query := `
		MERGE (fn:FileNumber {id: -1})
		ON CREATE SET fn.max_file_id = 1
		ON MATCH SET fn.max_file_id = fn.max_file_id + 1
		RETURN fn.max_file_id as next_file_id
	`

	record, err := cg.db.ExecuteWriteSingle(ctx, query, nil)
	if err != nil {
		cg.logger.Error("Failed to get or create next file ID", zap.Error(err))
		return 0, fmt.Errorf("failed to get or create next file ID: %w", err)
	}

	nextFileID, ok := record["next_file_id"]
	if !ok {
		return 0, fmt.Errorf("failed to get next_file_id from result")
	}

	// Handle different numeric types from different database backends
	switch v := nextFileID.(type) {
	case int32:
		return v, nil
	case int64:
		return int32(v), nil
	case int:
		return int32(v), nil
	default:
		return 0, fmt.Errorf("unexpected type for next_file_id: %T", nextFileID)
	}
}
*/

func (cg *CodeGraph) FindFunctionCalls(ctx context.Context, fileID ast.NodeID) (map[ast.NodeID][]*ast.Node, error) {
	query := `
		MATCH (fc:FunctionCall)<-[:CONTAINS*]-(f:Function)
		WHERE fc.fileId = $fileId
		RETURN fc, f.id AS functionId
	`

	parameters := map[string]any{
		"fileId": int64(fileID),
	}

	records, err := cg.db.ExecuteRead(ctx, query, parameters)
	if err != nil {
		cg.logger.Error("Failed to find function calls", zap.Error(err))
		return nil, fmt.Errorf("failed to find function calls: %w", err)
	}

	functionCalls := make(map[ast.NodeID][]*ast.Node)
	for _, record := range records {
		fcData, ok := record["fc"]
		if !ok || fcData == nil {
			continue
		}

		fcMap, ok := fcData.(map[string]any)
		if !ok {
			continue
		}

		node, err := cg.recordToNode(fcMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert record to node: %w", err)
		}

		functionId, ok := record["functionId"]
		if !ok {
			continue
		}

		functionCalls[ast.NodeID(functionId.(int64))] =
			append(functionCalls[ast.NodeID(functionId.(int64))], node)
	}

	return functionCalls, nil
}

func (cg *CodeGraph) FindFunctionsByName(ctx context.Context, fileID int, name string) ([]*ast.Node, error) {
	return cg.readNodes(ctx, ast.NodeTypeFunction, map[string]any{
		"name":   name,
		"fileId": fileID,
	})
}

// convertToInt64 safely converts various integer types to int64
func (cg *CodeGraph) convertToInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case uint64:
		return int64(v)
	case uint32:
		return int64(v)
	case uint:
		return int64(v)
	default:
		cg.logger.Warn("Unexpected type for int64 conversion", zap.Any("value", value), zap.String("type", fmt.Sprintf("%T", value)))
		return 0
	}
}

// convertToInt32 safely converts various integer types to int32
func (cg *CodeGraph) convertToInt32(value any) int32 {
	switch v := value.(type) {
	case int32:
		return v
	case int64:
		return int32(v)
	case int:
		return int32(v)
	case uint32:
		return int32(v)
	case uint64:
		return int32(v)
	case uint:
		return int32(v)
	default:
		cg.logger.Warn("Unexpected type for int32 conversion", zap.Any("value", value), zap.String("type", fmt.Sprintf("%T", value)))
		return 0
	}
}
