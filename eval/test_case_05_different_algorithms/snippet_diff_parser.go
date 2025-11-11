func ParseDiff(diffContent string) (*models.Patch, error) {
	if strings.TrimSpace(diffContent) == "" {
		return nil, fmt.Errorf("empty diff content")
	}

	files, _, err := gitdiff.Parse(strings.NewReader(diffContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	patch := &models.Patch{
		Raw:   diffContent,
		Files: make([]*models.FileChange, 0),
	}

	for _, file := range files {
		fileChange := convertFileToChange(file)
		if fileChange != nil {
			patch.Files = append(patch.Files, fileChange)
		}
	}

	return patch, nil
}

// convertFileToChange converts a gitdiff.File to a models.FileChange
func convertFileToChange(file *gitdiff.File) *models.FileChange {
	// Determine file path
	newPath := file.NewName
	oldPath := file.OldName

	if newPath == "" {
		newPath = oldPath
	}
	if oldPath == "" {
		oldPath = newPath
	}

	// Remove /dev/null paths and a/b/ prefixes
	if oldPath == "/dev/null" {
		oldPath = ""
	}
	if newPath == "/dev/null" {
		newPath = ""
	}

	// Strip a/ and b/ prefixes
	oldPath = strings.TrimPrefix(oldPath, "a/")
	newPath = strings.TrimPrefix(newPath, "b/")

	return &models.FileChange{
		OldPath: oldPath,
		NewPath: newPath,
		// ... more fields
	}
}
