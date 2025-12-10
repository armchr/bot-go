package util

import (
	"context"
	"fmt"

	"bot-go/internal/config"
)

// GitAnalyzer defines the interface for git history analysis
type GitAnalyzer interface {
	// GetRepoPath returns the repository path
	GetRepoPath() string

	// GetCoChangedClasses returns classes that frequently change together
	GetCoChangedClasses(ctx context.Context, classPath string, lookbackCommits int) ([]CoChangeInfo, error)

	// GetCoChangedMethods returns methods that frequently change together
	GetCoChangedMethods(ctx context.Context, methodPath string, lookbackCommits int) ([]CoChangeInfo, error)

	// GetFileChangeHistory returns the change history for a file
	GetFileChangeHistory(ctx context.Context, filePath string, lookbackCommits int) ([]ChangeInfo, error)

	// GetCoChangedFiles returns files that frequently change together
	GetCoChangedFiles(ctx context.Context, filePath string, lookbackCommits int) ([]CoChangeInfo, error)
}

// CoChangeInfo represents co-change information
type CoChangeInfo struct {
	EntityPath string   // Path to the co-changed entity
	Frequency  int      // Number of times changed together
	Commits    []string // Commit hashes where they changed together
}

// ChangeInfo represents a single change
type ChangeInfo struct {
	CommitHash   string
	Author       string
	Date         string
	Message      string
	LinesAdded   int
	LinesRemoved int
}

// NewGitAnalyzer creates a new GitAnalyzer based on configuration
// Currently only supports "ondemand" mode; "precompute" mode is not yet implemented
func NewGitAnalyzer(repoPath string, cfg *config.GitAnalysisConfig) (GitAnalyzer, error) {
	if cfg == nil {
		// Default to on-demand mode if no config provided
		return NewOnDemandGitAnalyzer(repoPath, 1000), nil
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("git analysis is disabled in configuration")
	}

	lookback := cfg.LookbackCommits
	if lookback <= 0 {
		lookback = 1000 // default
	}

	switch cfg.Mode {
	case config.GitAnalysisModeOnDemand, "": // empty string defaults to on-demand
		return NewOnDemandGitAnalyzer(repoPath, lookback), nil
	case config.GitAnalysisModePrecompute:
		return nil, fmt.Errorf("precompute mode for git analysis is not yet implemented")
	default:
		return nil, fmt.Errorf("unknown git analysis mode: %s", cfg.Mode)
	}
}

// OnDemandGitAnalyzer executes git commands on-demand when methods are called
type OnDemandGitAnalyzer struct {
	repoPath        string
	lookbackCommits int
}

// NewOnDemandGitAnalyzer creates a new on-demand git analyzer
func NewOnDemandGitAnalyzer(repoPath string, lookbackCommits int) *OnDemandGitAnalyzer {
	if lookbackCommits <= 0 {
		lookbackCommits = 1000
	}
	return &OnDemandGitAnalyzer{
		repoPath:        repoPath,
		lookbackCommits: lookbackCommits,
	}
}

// GetRepoPath returns the repository path
func (g *OnDemandGitAnalyzer) GetRepoPath() string {
	return g.repoPath
}

// GetCoChangedClasses returns classes that frequently change together
// For now, this delegates to GetCoChangedFiles since class-level tracking
// requires AST analysis of diffs
func (g *OnDemandGitAnalyzer) GetCoChangedClasses(ctx context.Context, classPath string, lookbackCommits int) ([]CoChangeInfo, error) {
	// TODO: Implement class-level co-change analysis
	// This would require:
	// 1. Getting commits that modified the file containing the class
	// 2. For each commit, parsing the diff to identify which classes changed
	// 3. Building co-change frequency matrix
	return nil, nil
}

// GetCoChangedMethods returns methods that frequently change together
func (g *OnDemandGitAnalyzer) GetCoChangedMethods(ctx context.Context, methodPath string, lookbackCommits int) ([]CoChangeInfo, error) {
	// TODO: Implement method-level co-change analysis
	// This would require AST diffing to identify method-level changes
	return nil, nil
}

// GetFileChangeHistory returns the change history for a file
func (g *OnDemandGitAnalyzer) GetFileChangeHistory(ctx context.Context, filePath string, lookbackCommits int) ([]ChangeInfo, error) {
	// TODO: Implement using git log
	// git log --follow -n {lookbackCommits} --pretty=format:"%H|%an|%ad|%s" --numstat -- {filePath}
	return nil, nil
}

// GetCoChangedFiles returns files that frequently change together
func (g *OnDemandGitAnalyzer) GetCoChangedFiles(ctx context.Context, filePath string, lookbackCommits int) ([]CoChangeInfo, error) {
	// TODO: Implement using git log
	// 1. git log --follow -n {lookbackCommits} --pretty=format:"%H" -- {filePath}  -> get commits
	// 2. For each commit: git diff-tree --no-commit-id --name-only -r {commit}  -> get co-changed files
	// 3. Aggregate and count frequencies
	return nil, nil
}

// Ensure OnDemandGitAnalyzer implements GitAnalyzer
var _ GitAnalyzer = (*OnDemandGitAnalyzer)(nil)
