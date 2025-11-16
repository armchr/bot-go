package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitInfo contains git repository information
type GitInfo struct {
	HeadCommitSHA  string
	HeadCommitMsg  string
	ModifiedFiles  map[string]bool // Set of files modified compared to HEAD
	IsGitRepo      bool
}

// GetGitInfo retrieves git information for a repository path
func GetGitInfo(repoPath string) (*GitInfo, error) {
	info := &GitInfo{
		ModifiedFiles: make(map[string]bool),
	}

	// Check if this is a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		info.IsGitRepo = false
		return info, nil
	}
	info.IsGitRepo = true

	// Get HEAD commit SHA
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit SHA: %w", err)
	}
	info.HeadCommitSHA = strings.TrimSpace(string(output))

	// Get HEAD commit message (first line)
	cmd = exec.Command("git", "log", "-1", "--pretty=%s")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit message: %w", err)
	}
	info.HeadCommitMsg = strings.TrimSpace(string(output))

	// Get modified files (compared to HEAD)
	// This includes: modified, added, deleted files in working directory and index
	cmd = exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get modified files: %w", err)
	}

	modifiedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, file := range modifiedFiles {
		if file != "" {
			// Convert to absolute path
			absPath := filepath.Join(repoPath, file)
			info.ModifiedFiles[absPath] = true
		}
	}

	return info, nil
}

// GetFileContentFromGit retrieves file content from git HEAD
func GetFileContentFromGit(repoPath, filePath string) ([]byte, error) {
	// Get relative path from repo root
	relPath, err := filepath.Rel(repoPath, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path: %w", err)
	}

	// Use git show to get file content from HEAD
	cmd := exec.Command("git", "show", fmt.Sprintf("HEAD:%s", relPath))
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get file content from git: %w", err)
	}

	return output, nil
}

// IsFileModified checks if a file is modified compared to HEAD
func IsFileModified(gitInfo *GitInfo, filePath string) bool {
	if gitInfo == nil || !gitInfo.IsGitRepo {
		return false
	}
	return gitInfo.ModifiedFiles[filePath]
}

// ReadFileOptimized reads file content, using git HEAD if useHead is true and file is unmodified
func ReadFileOptimized(repoPath, filePath string, useHead bool, gitInfo *GitInfo) ([]byte, error) {
	// If not using HEAD mode, read from disk
	if !useHead || gitInfo == nil || !gitInfo.IsGitRepo {
		return os.ReadFile(filePath)
	}

	// If file is modified compared to HEAD, read from disk
	if IsFileModified(gitInfo, filePath) {
		return os.ReadFile(filePath)
	}

	// File is unmodified, read from git HEAD for consistency
	return GetFileContentFromGit(repoPath, filePath)
}
