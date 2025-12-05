package controller

import (
	"context"
	"fmt"
	"net/http"

	"bot-go/internal/config"
	"bot-go/internal/service/codegraph"
	"bot-go/internal/service/ngram"
	"bot-go/internal/service/vector"
	"bot-go/internal/signals"
	"bot-go/internal/smells"
	"bot-go/internal/smells/godclass"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SmellController handles code smell detection HTTP endpoints
type SmellController struct {
	detectorRegistry *smells.DetectorRegistry
	extractor        *signals.ClassInfoExtractor
	config           *config.Config
	logger           *zap.Logger
}

// NewSmellController creates a new smell controller
func NewSmellController(
	codeGraph *codegraph.CodeGraph,
	vectorDB vector.VectorDatabase,
	ngramService *ngram.NGramService,
	config *config.Config,
	logger *zap.Logger,
) *SmellController {
	// Create class info extractor
	extractor := signals.NewClassInfoExtractor(codeGraph, vectorDB, ngramService, logger)

	// Create detector registry
	detectorRegistry := smells.NewDetectorRegistry(logger)

	// Register god class detector
	godClassDetector := godclass.NewGodClassDetector(logger)
	detectorRegistry.Register(godClassDetector)

	return &SmellController{
		detectorRegistry: detectorRegistry,
		extractor:        extractor,
		config:           config,
		logger:           logger,
	}
}

// DetectGodClassRequest is the request body for god class detection
type DetectGodClassRequest struct {
	RepoName               string `json:"repo_name" binding:"required"`
	ClassName              string `json:"class_name" binding:"required"`
	Strategy               string `json:"strategy"`                  // "rule_based", "score_based", "all" (default: "all")
	IncludeRecommendations bool   `json:"include_recommendations"`   // default: true
	IncludeMetricDetails   bool   `json:"include_metric_details"`    // default: true
}

// DetectGodClass handles POST /api/v1/detectGodClass
func (sc *SmellController) DetectGodClass(c *gin.Context) {
	var req DetectGodClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if req.Strategy == "" {
		req.Strategy = "all"
	}

	sc.logger.Info("Detecting god class",
		zap.String("repo", req.RepoName),
		zap.String("class", req.ClassName),
		zap.String("strategy", req.Strategy))

	ctx := context.Background()

	// Extract class info
	classInfo, err := sc.extractor.Extract(ctx, req.RepoName, req.ClassName)
	if err != nil {
		sc.logger.Error("Failed to extract class info",
			zap.String("class", req.ClassName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to extract class info: %v", err),
		})
		return
	}

	// Get god class detector
	detector, err := sc.detectorRegistry.Get("god_class_detector")
	if err != nil {
		sc.logger.Error("God class detector not found", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "God class detector not available",
		})
		return
	}

	// Run detection
	result, err := detector.Detect(ctx, classInfo)
	if err != nil {
		sc.logger.Error("Detection failed",
			zap.String("class", req.ClassName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Detection failed: %v", err),
		})
		return
	}

	// Build response
	response := buildDetectionResponse(result, req.IncludeMetricDetails, req.IncludeRecommendations)

	c.JSON(http.StatusOK, response)
}

// AnalyzeRepositoryRequest is the request body for repository analysis
type AnalyzeRepositoryRequest struct {
	RepoName    string `json:"repo_name" binding:"required"`
	Strategy    string `json:"strategy"`     // default: "score_based"
	MinSeverity string `json:"min_severity"` // "critical", "high", "medium", "low" (default: "medium")
	TopN        int    `json:"top_n"`        // Return top N god classes (default: 10, 0 = all)
}

// AnalyzeRepository handles POST /api/v1/analyzeRepository
func (sc *SmellController) AnalyzeRepository(c *gin.Context) {
	var req AnalyzeRepositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if req.Strategy == "" {
		req.Strategy = "score_based"
	}
	if req.MinSeverity == "" {
		req.MinSeverity = "medium"
	}
	if req.TopN == 0 {
		req.TopN = 10
	}

	sc.logger.Info("Analyzing repository",
		zap.String("repo", req.RepoName),
		zap.String("strategy", req.Strategy),
		zap.String("min_severity", req.MinSeverity))

	ctx := context.Background()

	// Extract all classes in the repository
	classes, err := sc.extractor.ExtractAll(ctx, req.RepoName)
	if err != nil {
		sc.logger.Error("Failed to extract classes",
			zap.String("repo", req.RepoName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to extract classes: %v", err),
		})
		return
	}

	// Get god class detector
	detector, err := sc.detectorRegistry.Get("god_class_detector")
	if err != nil {
		sc.logger.Error("God class detector not found", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "God class detector not available",
		})
		return
	}

	// Run detection on all classes
	var results []*smells.DetectionResult
	for _, classInfo := range classes {
		result, err := detector.Detect(ctx, classInfo)
		if err != nil {
			sc.logger.Warn("Detection failed for class",
				zap.String("class", classInfo.ClassName),
				zap.Error(err))
			continue
		}

		// Filter by severity
		if shouldIncludeResult(result, req.MinSeverity) {
			results = append(results, result)
		}
	}

	// Sort by severity and confidence
	sortResults(results)

	// Limit to top N
	if req.TopN > 0 && len(results) > req.TopN {
		results = results[:req.TopN]
	}

	// Build response
	response := buildRepositoryAnalysisResponse(req.RepoName, len(classes), results)

	c.JSON(http.StatusOK, response)
}

// Helper functions

func buildDetectionResponse(result *smells.DetectionResult, includeMetrics, includeRecommendations bool) map[string]interface{} {
	response := map[string]interface{}{
		"repo_name":        result.RepoName,
		"class_name":       result.ClassName,
		"file_path":        result.FilePath,
		"is_god_class":     result.IsSmell,
		"severity":         result.Severity,
		"confidence":       result.Confidence,
		"strategy":         result.Strategy,
		"violated_signals": result.ViolatedSignals,
		"detected_at":      result.DetectedAt,
	}

	if includeMetrics {
		response["metrics"] = result.SignalValues
	}

	if includeRecommendations {
		response["recommendations"] = result.Recommendations
	}

	return response
}

func buildRepositoryAnalysisResponse(repoName string, totalClasses int, results []*smells.DetectionResult) map[string]interface{} {
	// Count by severity
	severityCounts := map[string]int{
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
	}

	// Build simplified results
	simplifiedResults := make([]map[string]interface{}, 0, len(results))
	for _, result := range results {
		severityCounts[string(result.Severity)]++

		simplifiedResults = append(simplifiedResults, map[string]interface{}{
			"class_name":            result.ClassName,
			"file_path":             result.FilePath,
			"severity":              result.Severity,
			"confidence":            result.Confidence,
			"violated_signal_count": len(result.ViolatedSignals),
		})
	}

	return map[string]interface{}{
		"repo_name":         repoName,
		"total_classes":     totalClasses,
		"god_classes_found": len(results),
		"results":           simplifiedResults,
		"summary":           severityCounts,
	}
}

func shouldIncludeResult(result *smells.DetectionResult, minSeverity string) bool {
	if !result.IsSmell {
		return false
	}

	severityOrder := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
	}

	resultSeverity := severityOrder[string(result.Severity)]
	minSeverityLevel := severityOrder[minSeverity]

	return resultSeverity >= minSeverityLevel
}

func sortResults(results []*smells.DetectionResult) {
	// Simple bubble sort by severity (descending) then confidence (descending)
	severityOrder := map[smells.Severity]int{
		smells.SeverityCritical: 4,
		smells.SeverityHigh:     3,
		smells.SeverityMedium:   2,
		smells.SeverityLow:      1,
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			// Compare severity
			sev1 := severityOrder[results[i].Severity]
			sev2 := severityOrder[results[j].Severity]

			if sev2 > sev1 {
				results[i], results[j] = results[j], results[i]
			} else if sev2 == sev1 && results[j].Confidence > results[i].Confidence {
				// Same severity, sort by confidence
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
