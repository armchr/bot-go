// Check for print statements (possible debug code)
if strings.Contains(content, "print(") {
	issues = append(issues, models.NewIssue(
		file.NewPath,
		line.NewLine,
		models.SeverityMinor,
		models.CategoryCodeQuality,
		"Print statement",
		"Code contains print() which may be debugging code",
		"Use proper logging (logging module) instead of print",
		"static",
	))
}
