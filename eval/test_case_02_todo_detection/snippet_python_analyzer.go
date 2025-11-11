// Check for TODO/FIXME
if strings.Contains(content, "TODO") || strings.Contains(content, "FIXME") {
	issues = append(issues, models.NewIssue(
		file.NewPath,
		line.NewLine,
		models.SeverityMinor,
		models.CategoryMaintainability,
		"TODO/FIXME comment",
		"Code contains a TODO or FIXME comment",
		"Address the TODO/FIXME or create a tracking issue",
		"static",
	))
}
