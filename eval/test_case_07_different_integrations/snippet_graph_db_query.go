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

		// Handle different numeric types and increment
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

		// Update with new value
		updateQuery := "MATCH (fn:FileNumber {id: -1}) SET fn.max_file_id = $max_file_id RETURN fn.max_file_id as max_file_id"
		updateParams := map[string]any{"max_file_id": nextFileID}

		return db.executeQuery(ctx, updateQuery, updateParams)
	}

	// Node doesn't exist, create it
	nextFileID = 1
	createQuery := "CREATE (fn:FileNumber {id: -1, max_file_id: $max_file_id}) RETURN fn.max_file_id as max_file_id"
	createParams := map[string]any{"max_file_id": nextFileID}

	return db.executeQuery(ctx, createQuery, createParams)
}
