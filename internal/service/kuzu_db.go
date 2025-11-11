package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/kuzudb/go-kuzu"
	"go.uber.org/zap"
)

// KuzuDatabase implements the GraphDatabase interface using Kuzu
type KuzuDatabase struct {
	db     *kuzu.Database
	conn   *kuzu.Connection
	logger *zap.Logger
}

// NewKuzuDatabase creates a new Kuzu database instance
func NewKuzuDatabase(databasePath string, logger *zap.Logger) (*KuzuDatabase, error) {
	var db *kuzu.Database
	var err error

	if databasePath == ":memory:" || databasePath == "" {
		// Create in-memory database
		db, err = kuzu.OpenInMemoryDatabase(kuzu.DefaultSystemConfig())
	} else {
		// Create file-based database
		db, err = kuzu.OpenDatabase(databasePath, kuzu.DefaultSystemConfig())
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create Kuzu database: %w", err)
	}

	conn, err := kuzu.OpenConnection(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create Kuzu connection: %w", err)
	}

	kuzuDB := &KuzuDatabase{
		db:     db,
		conn:   conn,
		logger: logger,
	}

	// Initialize schema
	if err := kuzuDB.initializeSchema(); err != nil {
		kuzuDB.Close(context.Background())
		return nil, fmt.Errorf("failed to initialize Kuzu schema: %w", err)
	}

	return kuzuDB, nil
}

// VerifyConnectivity checks if the database connection is working
func (db *KuzuDatabase) VerifyConnectivity(ctx context.Context) error {
	// Test connectivity with a simple query
	result, err := db.conn.Query("RETURN 1")
	if err != nil {
		return fmt.Errorf("failed to verify Kuzu connectivity: %w", err)
	}
	result.Close()
	return nil
}

// Close closes the database connection
func (db *KuzuDatabase) Close(ctx context.Context) error {
	if db.conn != nil {
		db.conn.Close()
	}
	if db.db != nil {
		db.db.Close()
	}
	return nil
}

// ExecuteRead executes a read-only Cypher query and returns the raw records
func (db *KuzuDatabase) ExecuteRead(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	return db.executeQuery(ctx, query, params, false)
}

// ExecuteWrite executes a write Cypher query and returns the raw records
func (db *KuzuDatabase) ExecuteWrite(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	return db.executeQuery(ctx, query, params, true)
}

// ExecuteReadSingle executes a read-only Cypher query expecting a single record
func (db *KuzuDatabase) ExecuteReadSingle(ctx context.Context, query string, params map[string]any) (map[string]any, error) {
	records, err := db.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no records returned")
	}

	if len(records) > 1 {
		return nil, fmt.Errorf("expected single record, got %d", len(records))
	}

	return records[0], nil
}

// ExecuteWriteSingle executes a write Cypher query expecting a single record
func (db *KuzuDatabase) ExecuteWriteSingle(ctx context.Context, query string, params map[string]any) (map[string]any, error) {
	records, err := db.ExecuteWrite(ctx, query, params)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no records returned")
	}

	if len(records) > 1 {
		return nil, fmt.Errorf("expected single record, got %d", len(records))
	}

	return records[0], nil
}

// executeQuery is the internal method that executes queries and returns results
func (db *KuzuDatabase) executeQuery(ctx context.Context, query string, params map[string]any, isWrite bool) ([]map[string]any, error) {
	// Handle MERGE queries by converting them to Kuzu-compatible operations
	if isWrite && strings.Contains(strings.ToUpper(query), "MERGE") {
		return db.handleMergeQuery(ctx, query, params)
	}
	
	// Handle MATCH queries with labels for read operations
	if !isWrite && strings.Contains(strings.ToUpper(query), "MATCH") {
		query = db.convertMatchQuery(query)
	}

	var result *kuzu.QueryResult
	var err error

	if len(params) > 0 {
		// Use prepared statement for parameterized queries
		preparedStatement, err := db.conn.Prepare(query)
		if err != nil {
			db.logger.Error("Failed to prepare Kuzu query", 
				zap.String("query", query), 
				zap.Bool("isWrite", isWrite), 
				zap.Error(err))
			return nil, fmt.Errorf("failed to prepare query: %w", err)
		}
		defer preparedStatement.Close()

		result, err = db.conn.Execute(preparedStatement, params)
	} else {
		// Execute query directly
		result, err = db.conn.Query(query)
	}

	if err != nil {
		db.logger.Error("Failed to execute Kuzu query", 
			zap.String("query", query), 
			zap.Bool("isWrite", isWrite), 
			zap.Error(err))
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer result.Close()

	// Convert results to our standard format
	var records []map[string]any
	
	for result.HasNext() {
		tuple, err := result.Next()
		if err != nil {
			db.logger.Error("Failed to get next result row", zap.Error(err))
			return nil, fmt.Errorf("failed to get next result row: %w", err)
		}

		// Get the tuple as a map with column names as keys
		record, err := tuple.GetAsMap()
		if err != nil {
			db.logger.Error("Failed to convert tuple to map", zap.Error(err))
			return nil, fmt.Errorf("failed to convert tuple to map: %w", err)
		}
		
		// Convert any Kuzu-specific types to standard Go types
		convertedRecord := make(map[string]any)
		for key, value := range record {
			convertedRecord[key] = db.convertKuzuValue(value)
		}
		
		records = append(records, convertedRecord)
	}

	return records, nil
}

// convertKuzuValue converts Kuzu-specific types to standard Go types
func (db *KuzuDatabase) convertKuzuValue(value any) any {
	// Handle Kuzu Node objects by extracting their properties
	if node, ok := value.(kuzu.Node); ok {
		return node.Properties
	}
	
	// For other types, return as-is since Kuzu's GetAsMap() should already
	// return proper Go types. If needed, we can add specific type conversions here.
	return value
}

// KuzuNode wraps a Kuzu node to implement the GraphNode interface
type KuzuNode struct {
	node kuzu.Node
}

// GetProperties returns the node properties
func (n *KuzuNode) GetProperties() map[string]any {
	return n.node.Properties
}

// WrapKuzuNode wraps a Kuzu node in our GraphNode interface
func WrapKuzuNode(node kuzu.Node) GraphNode {
	return &KuzuNode{node: node}
}

// initializeSchema creates the necessary node and relationship tables for the CodeGraph
func (db *KuzuDatabase) initializeSchema() error {
	// Common fields template for all node types
	baseFields := `
			id INT64,
			nodeType INT64,
			fileId INT32,
			name STRING,
			range STRING,
			version INT32,
			scopeId INT64,
			metaData MAP(STRING, STRING),
			fake BOOLEAN,
			nameID STRING,
			return STRING,
			repo STRING,
			path STRING,
			PRIMARY KEY (id)`

	// Define node table schemas based on the CodeGraph node types
	nodeTableSchemas := []string{
		"CREATE NODE TABLE ModuleScope (" + baseFields + ")",
		"CREATE NODE TABLE FileScope (" + baseFields + ")",
		"CREATE NODE TABLE Block (" + baseFields + ")",
		"CREATE NODE TABLE Variable (" + baseFields + ")",
		"CREATE NODE TABLE Expression (" + baseFields + ")",
		"CREATE NODE TABLE Conditional (" + baseFields + ")",
		"CREATE NODE TABLE Function (" + baseFields + ")",
		"CREATE NODE TABLE Class (" + baseFields + ")",
		"CREATE NODE TABLE Field (" + baseFields + ")",
		"CREATE NODE TABLE FunctionCall (" + baseFields + ")",
		"CREATE NODE TABLE Loop (" + baseFields + ")",
		`CREATE NODE TABLE FileNumber (
			id INT64,
			max_file_id INT32,
			PRIMARY KEY (id)
		)`,
	}

	// Create all node tables
	for _, schema := range nodeTableSchemas {
		_, err := db.conn.Query(schema)
		if err != nil {
			db.logger.Error("Failed to create node table", zap.String("schema", schema), zap.Error(err))
			return fmt.Errorf("failed to create node table: %w", err)
		}
	}

	// For now, skip relationship creation as it's complex in Kuzu
	// We'll handle relationships through direct queries when needed
	// TODO: Add relationship tables as needed for specific use cases

	db.logger.Info("Successfully initialized Kuzu schema")
	return nil
}

// handleMergeQuery converts Neo4j-style MERGE queries to Kuzu-compatible operations
func (db *KuzuDatabase) handleMergeQuery(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	// Special case for FileNumber increment query
	if strings.Contains(query, "FileNumber") && strings.Contains(query, "max_file_id") {
		return db.handleFileNumberMerge(ctx, query, params)
	}
	
	// Parse the MERGE query to extract the node label and properties
	// This is a simplified implementation for the CodeGraph use case
	
	// Extract node label from MERGE (n:Label {id: $id})
	labelRegex := regexp.MustCompile(`MERGE\s*\(\s*\w+\s*:\s*(\w+)\s*\{[^}]*\}\s*\)`)
	labelMatches := labelRegex.FindStringSubmatch(query)
	if len(labelMatches) < 2 {
		return nil, fmt.Errorf("could not parse node label from MERGE query")
	}
	nodeLabel := labelMatches[1]
	
	// For CodeGraph, we know the structure - try to create the node
	// If it fails due to primary key constraint, we'll handle the error
	
	// Convert SET clause to CREATE clause
	// Extract the properties that should be set
	var createFields []string
	var createValues []any
	
	for key, value := range params {
		createFields = append(createFields, key)
		createValues = append(createValues, value)
	}
	
	// Build the CREATE query
	fieldPlaceholders := make([]string, len(createFields))
	for i, field := range createFields {
		fieldPlaceholders[i] = fmt.Sprintf("%s: $%s", field, field)
	}
	
	createQuery := fmt.Sprintf("CREATE (n:%s {%s})", 
		nodeLabel, 
		strings.Join(fieldPlaceholders, ", "))
	
	// Try to create the node using prepared statement with parameters
	preparedStatement, err := db.conn.Prepare(createQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare create query: %w", err)
	}
	defer preparedStatement.Close()
	
	result, err := db.conn.Execute(preparedStatement, params)
	if err != nil {
		// If creation failed due to primary key constraint, the node already exists
		// For now, just return empty result (equivalent to MERGE finding existing node)
		if strings.Contains(err.Error(), "PRIMARY KEY") || strings.Contains(err.Error(), "primary key") {
			db.logger.Debug("Node already exists, skipping creation", 
				zap.String("nodeLabel", nodeLabel),
				zap.Any("params", params))
			return []map[string]any{}, nil
		}
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	defer result.Close()
	
	// Return empty result for successful creation (MERGE doesn't return the created node in our use case)
	return []map[string]any{}, nil
}

// handleFileNumberMerge handles the special case of FileNumber increment
func (db *KuzuDatabase) handleFileNumberMerge(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	// Check if FileNumber node exists
	checkQuery := "MATCH (fn:FileNumber {id: -1}) RETURN fn.max_file_id as max_file_id"
	
	result, err := db.conn.Query(checkQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to check FileNumber existence: %w", err)
	}
	defer result.Close()
	
	var nextFileID int32
	
	if result.HasNext() {
		// Node exists, get current value and increment
		tuple, err := result.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to get FileNumber tuple: %w", err)
		}
		
		record, err := tuple.GetAsMap()
		if err != nil {
			return nil, fmt.Errorf("failed to convert FileNumber tuple to map: %w", err)
		}
		
		currentMax, ok := record["max_file_id"]
		if !ok {
			return nil, fmt.Errorf("max_file_id not found in FileNumber record")
		}
		
		// Handle different numeric types
		switch v := currentMax.(type) {
		case int32:
			nextFileID = v + 1
		case int64:
			nextFileID = int32(v) + 1
		case int:
			nextFileID = int32(v) + 1
		default:
			return nil, fmt.Errorf("unexpected type for max_file_id: %T", currentMax)
		}
		
		// Update the existing node
		updateQuery := "MATCH (fn:FileNumber {id: -1}) SET fn.max_file_id = $max_file_id"
		updateParams := map[string]any{"max_file_id": nextFileID}
		
		updateStmt, err := db.conn.Prepare(updateQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare update query: %w", err)
		}
		defer updateStmt.Close()
		
		updateResult, err := db.conn.Execute(updateStmt, updateParams)
		if err != nil {
			return nil, fmt.Errorf("failed to update FileNumber: %w", err)
		}
		updateResult.Close()
		
	} else {
		// Node doesn't exist, create it with initial value
		nextFileID = 1
		createQuery := "CREATE (fn:FileNumber {id: -1, max_file_id: $max_file_id})"
		createParams := map[string]any{"max_file_id": nextFileID}
		
		createStmt, err := db.conn.Prepare(createQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare create query: %w", err)
		}
		defer createStmt.Close()
		
		createResult, err := db.conn.Execute(createStmt, createParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create FileNumber: %w", err)
		}
		createResult.Close()
	}
	
	// Return the next file ID
	return []map[string]any{
		{"next_file_id": nextFileID},
	}, nil
}

// convertMatchQuery converts Neo4j-style MATCH queries to Kuzu format
func (db *KuzuDatabase) convertMatchQuery(query string) string {
	// Kuzu uses the same MATCH (n:Label) syntax as Neo4j, so we don't need to convert
	// The issue might be elsewhere. Let's keep the query as-is for now.
	db.logger.Debug("Converting match query", zap.String("original", query))
	return query
}