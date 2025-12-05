package size

import (
	"context"
	"strings"

	"bot-go/internal/signals"
)

// LOCSignal measures Lines of Code
type LOCSignal struct{}

// NewLOCSignal creates a new LOC signal
func NewLOCSignal() *LOCSignal {
	return &LOCSignal{}
}

func (s *LOCSignal) Name() string {
	return "LOC"
}

func (s *LOCSignal) Category() signals.SignalCategory {
	return signals.CategorySize
}

func (s *LOCSignal) Description() string {
	return "Lines of Code - total lines in the class (excluding blank lines and comments)"
}

func (s *LOCSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	// Use num_comment_lines from class node metadata if available
	if classInfo.ClassNode != nil && classInfo.ClassNode.MetaData != nil {
		if numCommentLines, ok := classInfo.ClassNode.MetaData["num_comment_lines"]; ok {
			// Convert to float64
			var commentLines float64
			switch v := numCommentLines.(type) {
			case int:
				commentLines = float64(v)
			case int32:
				commentLines = float64(v)
			case int64:
				commentLines = float64(v)
			case float64:
				commentLines = v
			case float32:
				commentLines = float64(v)
			}

			// Calculate: total lines - blank lines - comment lines
			totalLines := classInfo.EndLine - classInfo.StartLine + 1
			blankLines := s.countBlankLines(classInfo)
			loc := float64(totalLines) - float64(blankLines) - commentLines

			if loc < 0 {
				loc = 0
			}
			return loc, nil
		}
	}

	// Fallback: manual calculation if metadata not available
	return s.calculateManual(classInfo), nil
}

// countBlankLines counts blank lines in the class source code
func (s *LOCSignal) countBlankLines(classInfo *signals.ClassInfo) int {
	lines := strings.Split(string(classInfo.SourceCode), "\n")

	// Bounds check
	startIdx := classInfo.StartLine - 1 // Convert to 0-indexed
	endIdx := classInfo.EndLine

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	classLines := lines[startIdx:endIdx]

	blankLines := 0
	for _, line := range classLines {
		if strings.TrimSpace(line) == "" {
			blankLines++
		}
	}

	return blankLines
}

// calculateManual is the fallback manual calculation when metadata is not available
func (s *LOCSignal) calculateManual(classInfo *signals.ClassInfo) float64 {
	lines := strings.Split(string(classInfo.SourceCode), "\n")

	// Bounds check
	startIdx := classInfo.StartLine - 1 // Convert to 0-indexed
	endIdx := classInfo.EndLine

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	classLines := lines[startIdx:endIdx]

	nonBlankLines := 0
	inMultiLineComment := false

	for _, line := range classLines {
		trimmed := strings.TrimSpace(line)

		// Skip blank lines
		if trimmed == "" {
			continue
		}

		// Handle multi-line comments
		if strings.Contains(trimmed, "/*") {
			inMultiLineComment = true
		}
		if inMultiLineComment {
			if strings.Contains(trimmed, "*/") {
				inMultiLineComment = false
			}
			continue
		}

		// Skip single-line comments
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		nonBlankLines++
	}

	return float64(nonBlankLines)
}

// LOCNAMMSignal measures Lines of Code without Accessors/Mutators
type LOCNAMMSignal struct {
	locSignal *LOCSignal
}

// NewLOCNAMMSignal creates a new LOCNAMM signal
func NewLOCNAMMSignal() *LOCNAMMSignal {
	return &LOCNAMMSignal{
		locSignal: NewLOCSignal(),
	}
}

func (s *LOCNAMMSignal) Name() string {
	return "LOCNAMM"
}

func (s *LOCNAMMSignal) Category() signals.SignalCategory {
	return signals.CategorySize
}

func (s *LOCNAMMSignal) Description() string {
	return "Lines of Code without Accessors/Mutators - LOC excluding simple getters/setters"
}

func (s *LOCNAMMSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	// Get total LOC
	totalLOC, err := s.locSignal.Calculate(ctx, classInfo)
	if err != nil {
		return 0, err
	}

	// Calculate LOC for accessor methods, using metadata if available
	accessorLOC := 0.0
	for _, method := range classInfo.GetAccessorMethods() {
		methodLOC := s.calculateMethodLOC(method)
		accessorLOC += methodLOC
	}

	// Subtract accessor LOC from total
	result := totalLOC - accessorLOC
	if result < 0 {
		result = 0
	}

	return result, nil
}

// calculateMethodLOC calculates LOC for a method using metadata if available
func (s *LOCNAMMSignal) calculateMethodLOC(method *signals.MethodInfo) float64 {
	// Try to use num_comment_lines from method node metadata
	if method.Node != nil && method.Node.MetaData != nil {
		if numCommentLines, ok := method.Node.MetaData["num_comment_lines"]; ok {
			// Convert to float64
			var commentLines float64
			switch v := numCommentLines.(type) {
			case int:
				commentLines = float64(v)
			case int32:
				commentLines = float64(v)
			case int64:
				commentLines = float64(v)
			case float64:
				commentLines = v
			case float32:
				commentLines = float64(v)
			}

			// Calculate: total lines - blank lines - comment lines
			totalLines := method.EndLine - method.StartLine + 1
			blankLines := s.countMethodBlankLines(method)
			loc := float64(totalLines) - float64(blankLines) - commentLines

			if loc < 0 {
				loc = 0
			}
			return loc
		}
	}

	// Fallback: just use total lines (simple calculation)
	return float64(method.EndLine - method.StartLine + 1)
}

// countMethodBlankLines counts blank lines in a method's source code
func (s *LOCNAMMSignal) countMethodBlankLines(method *signals.MethodInfo) int {
	lines := strings.Split(string(method.SourceCode), "\n")
	blankLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankLines++
		}
	}
	return blankLines
}
