func (q *QdrantDatabase) SearchSimilar(ctx context.Context, collectionName string, queryVector []float32, limit int, filter map[string]interface{}) ([]*model.CodeChunk, []float32, error) {
	// Build Qdrant filter if provided
	var qdrantFilter *qdrant.Filter
	if len(filter) > 0 {
		conditions := make([]*qdrant.Condition, 0, len(filter))
		for key, value := range filter {
			conditions = append(conditions, &qdrant.Condition{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key:   key,
						Match: &qdrant.Match{MatchValue: &qdrant.Match_Keyword{Keyword: fmt.Sprint(value)}},
					},
				},
			})
		}
		qdrantFilter = &qdrant.Filter{
			Must: conditions,
		}
	}

	searchResult, err := q.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collectionName,
		Query:          qdrant.NewQuery(queryVector...),
		Limit:          qdrant.PtrOf(uint64(limit)),
		Filter:         qdrantFilter,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search: %w", err)
	}

	chunks := make([]*model.CodeChunk, 0, len(searchResult))
	scores := make([]float32, 0, len(searchResult))

	for _, point := range searchResult {
		chunk := pointToCodeChunk(point)
		if chunk != nil {
			chunks = append(chunks, chunk)
			scores = append(scores, point.Score)
		}
	}

	return chunks, scores, nil
}
