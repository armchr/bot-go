func (a *LLMAnalyzer) Analyze(ctx context.Context, file *models.FileChange) ([]*models.Issue, error) {
	issues := make([]*models.Issue, 0)

	// Get added lines
	addedLines := file.GetAddedLines()
	if len(addedLines) == 0 {
		return issues, nil
	}

	// Build code snippet with context
	codeSnippet := a.buildCodeSnippet(file)

	// Build prompt
	prompt := a.buildPrompt(file, codeSnippet)

	// Call LLM with retries
	var response string
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		response, err = a.client.Review(ctx, prompt)
		if err == nil {
			break
		}
		utils.Logger.Warnf("LLM request failed (attempt %d/%d): %v", i+1, maxRetries, err)
	}

	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed after %d attempts: %w", maxRetries, err)
	}

	// Parse response
	parsedIssues, err := a.parseResponse(response, file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return parsedIssues, nil
}
