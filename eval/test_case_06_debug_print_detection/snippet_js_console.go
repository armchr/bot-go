// Check for console.log (debugging code)
if strings.Contains(content, "console.log") || strings.Contains(content, "console.debug") {
	issues = append(issues, models.NewIssue(
		file.NewPath,
		line.NewLine,
		models.SeverityMinor,
		models.CategoryCodeQuality,
		"Console.log statement",
		"Code contains console.log which may be debugging code",
		"Remove console.log statements or use proper logging library",
		"static",
	))
}
